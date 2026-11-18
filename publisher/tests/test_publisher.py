from __future__ import annotations

import os
from pathlib import Path

import pytest
from excel.writer import SENS_GROWTH, SENS_WACC_STEPS
from model.pipeline import python_values

from publisher.serialize import sensitivity_grid, valuation_row
from publisher.db import connect, fetch_valuation, upsert_run


def test_sensitivity_grid_matches_wacc_growth_axes(aehr_model):
    v = python_values(aehr_model)
    grid = sensitivity_grid(aehr_model, float(v["WACC"]))
    assert len(grid) == len(SENS_WACC_STEPS) * len(SENS_GROWTH)
    center = next(
        c
        for c in grid
        if abs(c["wacc"] - v["WACC"]) < 1e-12 and abs(c["terminal_growth"] - 0.03) < 1e-12
    )
    assert center["value_per_share"] == pytest.approx(v["Base"], rel=1e-6)


def test_valuation_row_from_captured_aehr(aehr_model):
    row = valuation_row(aehr_model)
    assert row["ticker"] == "AEHR"
    assert row["as_of"] == "2026-10-14"
    assert row["peer_tickers"] == ["TER", "COHU", "FORM", "KLIC"]
    assert row["current_price"] and row["current_price"] > 0
    assert row["wacc"] and 0 < row["wacc"] < 1
    assert row["beta_ci_low"] < row["beta"] < row["beta_ci_high"]
    assert isinstance(row["data_notes"], list)
    assert "peers" in row["comps"]
    assert len(row["sensitivity"]) == 45


@pytest.mark.skipif(not os.environ.get("TEST_DATABASE_URL"), reason="TEST_DATABASE_URL not set")
def test_upsert_replaces_same_day(aehr_model, tmp_path: Path):
    row = valuation_row(aehr_model)
    conn = connect(os.environ["TEST_DATABASE_URL"])
    try:
        upsert_run(conn, row, str(tmp_path / "a.xlsx"))
        row2 = dict(row)
        row2["current_price"] = 999.0
        upsert_run(conn, row2, str(tmp_path / "b.xlsx"))
        got = fetch_valuation(conn, "AEHR", row["as_of"])
        assert got is not None
        assert len(got["sensitivity"]) == 45
        with conn.cursor() as cur:
            cur.execute("SELECT current_price, COUNT(*) OVER() FROM valuations WHERE ticker='AEHR' AND as_of=%s", (row["as_of"],))
            price, n = cur.fetchone()
        assert float(price) == 999.0
        assert n == 1
    finally:
        conn.close()
