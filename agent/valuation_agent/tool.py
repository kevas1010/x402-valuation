from __future__ import annotations

import json
from typing import Any, Callable, Protocol
from urllib.parse import urljoin, urlparse

from valuation_agent.guardrails import SpendGuard, SpendRefused

TOOL_NAME = "buy_valuation"
TOOL_SPEC = {
    "name": TOOL_NAME,
    "description": (
        "Pay for and fetch one valuation API route from the catalog. "
        "Use catalog ids: valuation, dcf, comps, compare, workbook. "
        "Pass ticker for single-name routes, or tickers as a comma-separated list for compare."
    ),
    "input_schema": {
        "type": "object",
        "properties": {
            "route": {"type": "string", "description": "Catalog route id, e.g. valuation or dcf"},
            "params": {
                "type": "object",
                "description": "Path/query params: ticker, or tickers for compare",
            },
        },
        "required": ["route"],
    },
}


class HTTPClient(Protocol):
    def get(self, url: str) -> tuple[int, str, dict[str, str]]: ...


class FakeSignerHTTP:
    """Test double. Records whether a paid GET was attempted."""

    def __init__(self, responses: dict[str, tuple[int, str]] | None = None):
        self.signed_gets: list[str] = []
        self.responses = responses or {}

    def get(self, url: str) -> tuple[int, str, dict[str, str]]:
        self.signed_gets.append(url)
        for key, (code, body) in self.responses.items():
            if key in url:
                return code, body, {}
        return 200, "{}", {}


def parse_price(raw: str) -> float:
    return float(str(raw).replace("$", "").strip())


class ValuationBuyer:
    def __init__(self, base_url: str, guard: SpendGuard, http: HTTPClient, catalog_http: HTTPClient | None = None):
        self.base_url = base_url.rstrip("/")
        self.guard = guard
        self.http = http
        self.catalog_http = catalog_http or http
        self._catalog: dict[str, Any] | None = None

    def catalog(self) -> dict[str, Any]:
        if self._catalog is None:
            url = urljoin(self.base_url + "/", "v1/catalog")
            code, body, _ = self.catalog_http.get(url)
            if code != 200:
                raise RuntimeError(f"catalog HTTP {code}: {body}")
            self._catalog = json.loads(body)
        return self._catalog

    def buy(self, route: str, params: dict[str, Any] | None = None) -> dict[str, Any]:
        params = params or {}
        cat = self.catalog()
        entry = self._lookup(route, cat)
        url = self._build_url(entry["path"], params)
        price = parse_price(entry["price"])
        try:
            self.guard.check(url, price)
        except SpendRefused as e:
            return {"error": e.reason, "paid": False, "refused": True}
        code, body, headers = self.http.get(url)
        self.guard.record(url, price, route)
        payload: dict[str, Any]
        try:
            payload = json.loads(body) if body else {}
        except json.JSONDecodeError:
            payload = {"raw": body}
        return {
            "error": None,
            "paid": True,
            "refused": False,
            "status": code,
            "price": price,
            "url": url,
            "body": payload,
            "as_of": payload.get("as_of") or headers.get("X-As-Of"),
            "data_notes": payload.get("data_notes", []),
        }

    def _lookup(self, route: str, cat: dict[str, Any]) -> dict[str, Any]:
        rid = str(route).strip().lower()
        for r in cat.get("routes", []):
            if str(r.get("id", "")).lower() == rid or str(r.get("path", "")).lower() == rid:
                return r
        raise SpendRefused(f"unknown route {route!r}") if False else KeyError(f"unknown route {route!r}")

    def _build_url(self, path: str, params: dict[str, Any]) -> str:
        filled = path
        if "{ticker}" in filled:
            ticker = str(params.get("ticker", "")).upper()
            filled = filled.replace("{ticker}", ticker)
        query = ""
        if "tickers" in params and params["tickers"]:
            query = "?tickers=" + str(params["tickers"]).replace(" ", "")
        return self.base_url + filled + query
