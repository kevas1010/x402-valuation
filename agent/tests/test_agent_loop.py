from __future__ import annotations

import json
from types import SimpleNamespace

from valuation_agent.agent import ValuationAgent
from valuation_agent.guardrails import SpendGuard
from valuation_agent.tool import FakeSignerHTTP, TOOL_NAME, ValuationBuyer


class FakeLLM:
    def __init__(self):
        self.n = 0
        self.messages = SimpleNamespace()
        self.messages.create = self.create

    def create(self, **kwargs):
        self.n += 1
        if self.n == 1:
            return SimpleNamespace(
                usage=SimpleNamespace(input_tokens=10, output_tokens=5),
                content=[
                    SimpleNamespace(
                        type="tool_use",
                        id="1",
                        name=TOOL_NAME,
                        input={"route": "valuation", "params": {"ticker": "AEHR"}},
                    )
                ],
            )
        return SimpleNamespace(
            usage=SimpleNamespace(input_tokens=8, output_tokens=20),
            content=[SimpleNamespace(type="text", text="AEHR as_of 2026-10-14. Data Notes: none. Model output, not investment advice. Testnet only.")],
        )


def test_multi_call_telemetry():
    catalog = {
        "routes": [{"id": "valuation", "path": "/v1/valuations/{ticker}", "price": "$0.002"}],
        "covered_tickers": ["AEHR"],
    }
    catalog_http = FakeSignerHTTP({"catalog": (200, json.dumps(catalog))})
    http = FakeSignerHTTP(
        {"valuations": (200, json.dumps({"as_of": "2026-10-14", "data_notes": [], "current_price": 20}))}
    )
    buyer = ValuationBuyer("http://localhost:8080", SpendGuard("localhost:8080"), http, catalog_http=catalog_http)
    agent = ValuationAgent(buyer, FakeLLM(), "claude-haiku-4-5")
    r = agent.ask("Is AEHR cheap?")
    assert "2026-10-14" in r.answer
    assert r.usdc_spent == 0.002
    assert r.input_tokens == 18
    assert r.output_tokens == 25
    assert r.llm_cost_usd > 0
    assert r.tool_calls[0]["name"] == TOOL_NAME
