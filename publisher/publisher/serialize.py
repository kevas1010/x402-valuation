"""Map a ModelData run to DB rows. No valuation math lives here besides the existing pipeline."""

from __future__ import annotations

import math
from typing import Any

import numpy as np
from excel.writer import SENS_GROWTH, SENS_WACC_STEPS, ModelData, write_workbook
from model.assumptions import revenue_cagr
from model.comps import MULTIPLES, implied_value_per_share, multiples_table
from model.dcf import dcf
from model.pipeline import python_values
from edgar.extract import with_derived

Z_95 = 1.96


def _finite(v: Any) -> float | None:
    if v is None:
        return None
    try:
        x = float(v)
    except (TypeError, ValueError):
        return None
    if math.isnan(x) or math.isinf(x):
        return None
    return x


def notes_json(data: ModelData) -> list[dict[str, str]]:
    return [{"category": n.category, "item": n.item, "message": n.message} for n in data.fin.notes]


def _base_dcf(data: ModelData, discount: float, terminal_growth: float):
    fin, a = data.fin, data.assumptions
    debt, cash = fin.latest("Debt"), fin.latest("Cash")
    return dcf(
        fin.latest("Revenue"),
        a.revenue_growth,
        a.ebitda_margin,
        a.da_pct,
        a.capex_pct,
        a.tax_rate,
        discount,
        terminal_growth,
        debt,
        cash,
        fin.shares_outstanding,
        a.terminal_capex_equals_da,
    )


def sensitivity_grid(data: ModelData, wacc: float) -> list[dict[str, float | None]]:
    rows: list[dict[str, float | None]] = []
    for step in SENS_WACC_STEPS:
        w = wacc + step
        for g in SENS_GROWTH:
            if w <= g:
                value = None
            else:
                value = _finite(_base_dcf(data, w, g).value_per_share)
            rows.append({"wacc": w, "terminal_growth": g, "value_per_share": value})
    return rows


def comps_payload(data: ModelData) -> dict[str, Any]:
    fin = data.fin
    t = with_derived(fin.table)
    table = multiples_table(data.peers) if data.peers else None
    peers = []
    excluded: list[dict[str, str]] = []
    for p in data.peers:
        row = {
            "ticker": p.ticker,
            "name": p.name,
            "ev_revenue": _finite(p.multiple("EV/Revenue")),
            "ev_ebitda": _finite(p.multiple("EV/EBITDA")),
            "pe": _finite(p.multiple("P/E")),
        }
        peers.append(row)
        for name in MULTIPLES:
            raw = p.raw_multiple(name)
            used = p.multiple(name)
            if np.isnan(used) and not np.isnan(raw):
                excluded.append({"ticker": p.ticker, "multiple": name, "reason": "outlier cap"})
            elif np.isnan(used):
                excluded.append({"ticker": p.ticker, "multiple": name, "reason": "not meaningful"})
    medians = means = {}
    if table is not None and "Median" in table.index:
        medians = {k: _finite(table.loc["Median", k]) for k in MULTIPLES}
        means = {k: _finite(table.loc["Mean", k]) for k in MULTIPLES}
    implied = implied_value_per_share(
        data.peers,
        fin.latest("Revenue"),
        t.loc["EBITDA"].iloc[-1],
        fin.latest("Net Income"),
        fin.latest("Debt"),
        fin.latest("Cash"),
        fin.shares_outstanding,
    )
    return {
        "peers": peers,
        "medians": medians,
        "means": means,
        "excluded_outliers": excluded,
        "implied_value_per_share": {k: _finite(v) for k, v in implied.items()},
    }


def valuation_row(data: ModelData) -> dict[str, Any]:
    v = python_values(data)
    wacc = float(v["WACC"])
    price = float(data.price)
    target = _finite(v["Target"])
    upside = None if target is None or not price else (target / price - 1.0)
    ci_half = Z_95 * float(data.beta.std_err)
    hist = with_derived(data.fin.table)
    cagr = revenue_cagr(hist.loc["Revenue"])
    base_dcf = _base_dcf(data, wacc, data.assumptions.terminal_growth)
    return {
        "ticker": data.fin.ticker,
        "as_of": data.as_of.isoformat(),
        "current_price": _finite(price),
        "dcf_value_per_share": _finite(v["Base"]),
        "comps_value_per_share": _finite(v["Comps"]),
        "probability_weighted_price_target": target,
        "bear_target": _finite(v["Bear"]),
        "base_target": _finite(v["Base"]),
        "bull_target": _finite(v["Bull"]),
        "wacc": _finite(wacc),
        "beta": _finite(data.beta.beta),
        "beta_r_squared": _finite(data.beta.r_squared),
        "beta_ci_low": _finite(data.beta.beta - ci_half),
        "beta_ci_high": _finite(data.beta.beta + ci_half),
        "terminal_growth": _finite(data.assumptions.terminal_growth),
        "revenue_cagr": _finite(cagr),
        "peer_tickers": list(data.peer_tickers),
        "data_notes": notes_json(data),
        "upside_pct": _finite(upside),
        "dcf_detail": {
            "wacc": _finite(wacc),
            "beta": _finite(data.beta.beta),
            "beta_r_squared": _finite(data.beta.r_squared),
            "beta_ci_low": _finite(data.beta.beta - ci_half),
            "beta_ci_high": _finite(data.beta.beta + ci_half),
            "n_obs": data.beta.n_obs,
            "revenue_growth": [_finite(g) for g in data.assumptions.revenue_growth],
            "terminal_growth": _finite(data.assumptions.terminal_growth),
            "terminal_value": _finite(base_dcf.terminal_value),
            "enterprise_value": _finite(base_dcf.enterprise_value),
        },
        "comps": comps_payload(data),
        "sensitivity": sensitivity_grid(data, wacc),
    }


def write_xlsx(data: ModelData, out_dir) -> str:
    path = write_workbook(data, out_dir / f"{data.fin.ticker}_valuation_{data.as_of.isoformat()}.xlsx")
    return str(path)
