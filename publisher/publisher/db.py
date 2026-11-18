from __future__ import annotations

import json
from typing import Any

import psycopg
from psycopg.types.json import Jsonb


UPSERT_VALUATION = """
INSERT INTO valuations (
    ticker, as_of, current_price, dcf_value_per_share, comps_value_per_share,
    probability_weighted_price_target, bear_target, base_target, bull_target,
    wacc, beta, beta_r_squared, beta_ci_low, beta_ci_high, terminal_growth,
    revenue_cagr, peer_tickers, data_notes, dcf_detail, comps
) VALUES (
    %(ticker)s, %(as_of)s, %(current_price)s, %(dcf_value_per_share)s, %(comps_value_per_share)s,
    %(probability_weighted_price_target)s, %(bear_target)s, %(base_target)s, %(bull_target)s,
    %(wacc)s, %(beta)s, %(beta_r_squared)s, %(beta_ci_low)s, %(beta_ci_high)s, %(terminal_growth)s,
    %(revenue_cagr)s, %(peer_tickers)s, %(data_notes)s, %(dcf_detail)s, %(comps)s
)
ON CONFLICT (ticker, as_of) DO UPDATE SET
    current_price = EXCLUDED.current_price,
    dcf_value_per_share = EXCLUDED.dcf_value_per_share,
    comps_value_per_share = EXCLUDED.comps_value_per_share,
    probability_weighted_price_target = EXCLUDED.probability_weighted_price_target,
    bear_target = EXCLUDED.bear_target,
    base_target = EXCLUDED.base_target,
    bull_target = EXCLUDED.bull_target,
    wacc = EXCLUDED.wacc,
    beta = EXCLUDED.beta,
    beta_r_squared = EXCLUDED.beta_r_squared,
    beta_ci_low = EXCLUDED.beta_ci_low,
    beta_ci_high = EXCLUDED.beta_ci_high,
    terminal_growth = EXCLUDED.terminal_growth,
    revenue_cagr = EXCLUDED.revenue_cagr,
    peer_tickers = EXCLUDED.peer_tickers,
    data_notes = EXCLUDED.data_notes,
    dcf_detail = EXCLUDED.dcf_detail,
    comps = EXCLUDED.comps
"""

REPLACE_SENS = "DELETE FROM sensitivity WHERE ticker = %s AND as_of = %s"
INSERT_SENS = """
INSERT INTO sensitivity (ticker, as_of, wacc, terminal_growth, value_per_share)
VALUES (%s, %s, %s, %s, %s)
"""
UPSERT_WB = """
INSERT INTO workbooks (ticker, as_of, path) VALUES (%s, %s, %s)
ON CONFLICT (ticker, as_of) DO UPDATE SET path = EXCLUDED.path
"""


def connect(dsn: str) -> psycopg.Connection:
    return psycopg.connect(dsn, autocommit=False)


def upsert_run(conn: psycopg.Connection, row: dict[str, Any], workbook_path: str) -> None:
    params = {
        **row,
        "data_notes": Jsonb(row["data_notes"]),
        "dcf_detail": Jsonb(row["dcf_detail"]),
        "comps": Jsonb(row["comps"]),
    }
    del params["sensitivity"]
    del params["upside_pct"]
    with conn.cursor() as cur:
        cur.execute(UPSERT_VALUATION, params)
        cur.execute(REPLACE_SENS, (row["ticker"], row["as_of"]))
        for cell in row["sensitivity"]:
            cur.execute(
                INSERT_SENS,
                (row["ticker"], row["as_of"], cell["wacc"], cell["terminal_growth"], cell["value_per_share"]),
            )
        cur.execute(UPSERT_WB, (row["ticker"], row["as_of"], workbook_path))
    conn.commit()


def fetch_valuation(conn: psycopg.Connection, ticker: str, as_of: str) -> dict[str, Any] | None:
    with conn.cursor() as cur:
        cur.execute(
            "SELECT ticker, as_of, wacc, data_notes, comps FROM valuations WHERE ticker = %s AND as_of = %s",
            (ticker, as_of),
        )
        rec = cur.fetchone()
        if rec is None:
            return None
        cur.execute(
            "SELECT wacc, terminal_growth, value_per_share FROM sensitivity WHERE ticker = %s AND as_of = %s ORDER BY wacc, terminal_growth",
            (ticker, as_of),
        )
        grid = cur.fetchall()
    return {
        "ticker": rec[0],
        "as_of": rec[1].isoformat(),
        "wacc": rec[2],
        "data_notes": rec[3] if not isinstance(rec[3], str) else json.loads(rec[3]),
        "comps": rec[4] if not isinstance(rec[4], str) else json.loads(rec[4]),
        "sensitivity": [
            {"wacc": r[0], "terminal_growth": r[1], "value_per_share": r[2]} for r in grid
        ],
    }
