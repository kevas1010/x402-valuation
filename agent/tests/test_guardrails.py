from __future__ import annotations

import json

import pytest

from valuation_agent.guardrails import SpendGuard, SpendRefused
from valuation_agent.tool import FakeSignerHTTP, ValuationBuyer


CATALOG = {
    "routes": [
        {"id": "valuation", "path": "/v1/valuations/{ticker}", "price": "$0.002"},
        {"id": "dcf", "path": "/v1/valuations/{ticker}/dcf", "price": "$0.003"},
        {"id": "workbook", "path": "/v1/workbooks/{ticker}", "price": "$0.01"},
        {"id": "compare", "path": "/v1/compare", "price": "$0.005"},
        {"id": "pricey", "path": "/v1/valuations/{ticker}/secret", "price": "$0.05"},
    ],
    "covered_tickers": ["AEHR", "TER"],
}


def buyer(max_price=0.01, budget=0.03, host="localhost:8080"):
    catalog_http = FakeSignerHTTP({"catalog": (200, json.dumps(CATALOG))})
    http = FakeSignerHTTP({"v1/": (200, json.dumps({"as_of": "2026-10-14", "data_notes": []}))})
    guard = SpendGuard(allowed_host=host, max_price=max_price, question_budget=budget)
    b = ValuationBuyer("http://localhost:8080", guard, http, catalog_http=catalog_http)
    return b, http


def test_over_max_price_refused_before_sign():
    b, http = buyer()
    out = b.buy("pricey", {"ticker": "AEHR"})
    assert out["refused"] is True
    assert "max per call" in out["error"]
    assert http.signed_gets == []


def test_non_allowlisted_host_refused_before_sign():
    guard = SpendGuard(allowed_host="api.example.test", max_price=0.01, question_budget=0.03)
    http = FakeSignerHTTP()
    b = ValuationBuyer("http://evil.test", guard, http)
    b._catalog = CATALOG
    out = b.buy("valuation", {"ticker": "AEHR"})
    assert out["refused"] is True
    assert "allowlisted" in out["error"]
    assert http.signed_gets == []


def test_over_budget_refused_before_sign():
    b, http = buyer(budget=0.004)
    out = b.buy("dcf", {"ticker": "AEHR"})
    assert out["paid"] is True
    out2 = b.buy("dcf", {"ticker": "TER"})
    assert out2["refused"] is True
    assert "question budget" in out2["error"]
    assert len(http.signed_gets) == 1


def test_spend_telemetry_adds_up():
    b, http = buyer()
    b.buy("valuation", {"ticker": "AEHR"})
    b.buy("dcf", {"ticker": "AEHR"})
    b.buy("compare", {"tickers": "AEHR,TER"})
    assert b.guard.spent == pytest.approx(0.002 + 0.003 + 0.005)
    assert len(b.guard.calls) == 3
    assert len(http.signed_gets) == 3


def test_check_raises():
    g = SpendGuard(allowed_host="localhost:8080", max_price=0.01, question_budget=0.03)
    with pytest.raises(SpendRefused):
        g.check("http://other:8080/v1/valuations/AEHR", 0.002)
