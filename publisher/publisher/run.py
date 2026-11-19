from __future__ import annotations

import os
from datetime import date
from functools import partial
from pathlib import Path

from edgar.client import EdgarClient
from excel.writer import write_workbook
from model.pipeline import build_model

from publisher.db import connect, upsert_run
from publisher.serialize import valuation_row
from publisher.tickers import DEFAULT_PEERS, parse_ticker_list


def run_ticker(ticker: str, client: EdgarClient, price_loader, as_of: date | None, out_dir: Path):
    peers = DEFAULT_PEERS.get(ticker, [])
    data = build_model(ticker, peers, client, price_loader=price_loader, as_of=as_of, verbose=True)
    row = valuation_row(data)
    out_dir.mkdir(parents=True, exist_ok=True)
    path = write_workbook(data, out_dir / f"{data.fin.ticker}_valuation_{data.as_of.isoformat()}.xlsx")
    return row, str(path)


def main() -> None:
    from model.prices import get_prices

    dsn = os.environ["DATABASE_URL"]
    out_dir = Path(os.environ.get("WORKBOOK_DIR", "workbooks"))
    tickers = parse_ticker_list(os.environ.get("TICKERS"))
    offline = os.environ.get("EDGAR_OFFLINE") == "1"
    cache = os.environ.get("EDGAR_CACHE")
    client_kw = {}
    if cache:
        client_kw["cache_dir"] = cache
    client = EdgarClient(offline=offline, **client_kw)
    as_of = date.fromisoformat(os.environ["AS_OF"]) if os.environ.get("AS_OF") else None
    price_kw = {}
    if os.environ.get("PRICE_CACHE"):
        price_kw["cache_dir"] = os.environ["PRICE_CACHE"]
    if as_of is not None:
        price_kw["as_of"] = as_of
    if offline:
        price_kw["offline"] = True
    price_loader = partial(get_prices, **price_kw) if price_kw else get_prices

    conn = connect(dsn)
    try:
        for ticker in tickers:
            print(f"=== {ticker} ===")
            row, path = run_ticker(ticker, client, price_loader, as_of, out_dir)
            upsert_run(conn, row, path)
            print(f"upserted {ticker} as_of={row['as_of']} workbook={path}")
    finally:
        conn.close()
