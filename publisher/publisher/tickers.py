"""Covered universe. Peers are the same sets used when the EDGAR model was tested."""

from __future__ import annotations

# 20 tickers. AEHR / TER / MSFT keep the peer lists from the EDGAR README.
DEFAULT_PEERS: dict[str, list[str]] = {
    "AEHR": ["TER", "COHU", "FORM", "KLIC"],
    "TER": ["AEHR", "COHU", "FORM", "KLIC"],
    "COHU": ["TER", "AEHR", "FORM", "KLIC"],
    "FORM": ["TER", "COHU", "AEHR", "KLIC"],
    "KLIC": ["TER", "COHU", "FORM", "AEHR"],
    "AMAT": ["LRCX", "KLAC", "ASML"],
    "KLAC": ["AMAT", "LRCX", "ASML"],
    "LRCX": ["AMAT", "KLAC", "ASML"],
    "MSFT": ["GOOGL", "ORCL", "AMZN", "CRM"],
    "GOOGL": ["MSFT", "AMZN", "META", "ORCL"],
    "ORCL": ["MSFT", "GOOGL", "AMZN", "CRM"],
    "AMZN": ["MSFT", "GOOGL", "META", "ORCL"],
    "CRM": ["MSFT", "ORCL", "NOW", "ADBE"],
    "AAPL": ["MSFT", "GOOGL", "AMZN", "META"],
    "NVDA": ["AMD", "AVGO", "TSM", "INTC"],
    "AMD": ["NVDA", "INTC", "AVGO", "QCOM"],
    "INTC": ["AMD", "NVDA", "TSM", "QCOM"],
    "QCOM": ["AVGO", "TXN", "AMD", "INTC"],
    "AVGO": ["QCOM", "TXN", "NVDA", "AMD"],
    "TXN": ["ADBE", "QCOM", "AVGO", "ADI"],
}

DEFAULT_TICKERS = list(DEFAULT_PEERS)


def parse_ticker_list(raw: str | None) -> list[str]:
    if not raw:
        return list(DEFAULT_TICKERS)
    return [t.strip().upper() for t in raw.split(",") if t.strip()]
