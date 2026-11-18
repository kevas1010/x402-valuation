"""Reuse the EDGAR project's offline fixtures (captured 2026-10-14)."""
from __future__ import annotations

import sys
from datetime import date, datetime
from functools import partial
from pathlib import Path

import pytest

ROOT = Path(__file__).resolve().parents[2]
EDGAR = ROOT / "vendor" / "edgar-valuation"
sys.path.insert(0, str(EDGAR))
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from edgar.client import EdgarClient  # noqa: E402
from model.pipeline import build_model  # noqa: E402
from model.prices import get_prices  # noqa: E402

FIXTURES = EDGAR / "tests" / "fixtures"
FIXTURE_DATE = date(2026, 10, 14)
PEERS = ["TER", "COHU", "FORM", "KLIC"]


@pytest.fixture(scope="session")
def client() -> EdgarClient:
    return EdgarClient(cache_dir=FIXTURES, offline=True)


@pytest.fixture(scope="session")
def price_loader():
    return partial(get_prices, cache_dir=FIXTURES / "prices", as_of=FIXTURE_DATE, offline=True)


@pytest.fixture(scope="session")
def aehr_model(client, price_loader):
    return build_model(
        "AEHR", PEERS, client, price_loader, as_of=FIXTURE_DATE, pulled_at=datetime(2026, 10, 14, 12, 0)
    )
