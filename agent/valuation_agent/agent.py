from __future__ import annotations

import argparse
import json
import os
import sys
import time
from dataclasses import dataclass, field
from pathlib import Path
from urllib.parse import urlparse

from dotenv import load_dotenv

from valuation_agent.guardrails import SpendGuard, SpendRefused
from valuation_agent.tool import TOOL_NAME, TOOL_SPEC, ValuationBuyer, parse_price

ROOT = Path(__file__).resolve().parents[1]
load_dotenv(ROOT / ".env")

PRICING = {
    "claude-opus-4-5": (5.00, 25.00),
    "claude-sonnet-4-5": (3.00, 15.00),
    "claude-haiku-4-5": (1.00, 5.00),
    "claude-haiku-4-5-20251001": (1.00, 5.00),
}

MAX_ITERATIONS = 8

SYSTEM = """You are a research assistant using a paid EDGAR equity-valuation API (testnet only).

Rules:
- Use buy_valuation to fetch data. Never invent prices, WACC, or targets.
- Cite the as_of date from every payload you use.
- Mention Data Notes (missing-data warnings) when they are present.
- If a tool call is refused because of a spending cap, explain the cap and answer with what you already have.
- End with: Model output, not investment advice. Testnet only.
Keep answers short: the number first, then one or two sentences.
"""


@dataclass
class AgentResult:
    question: str
    answer: str
    tool_calls: list[dict] = field(default_factory=list)
    latency_s: float = 0.0
    input_tokens: int = 0
    output_tokens: int = 0
    usdc_spent: float = 0.0
    model: str = ""

    @property
    def llm_cost_usd(self) -> float:
        pin, pout = PRICING.get(self.model, PRICING["claude-haiku-4-5"])
        return (self.input_tokens * pin + self.output_tokens * pout) / 1_000_000


class ValuationAgent:
    def __init__(self, buyer: ValuationBuyer, llm, model: str):
        self.buyer = buyer
        self.llm = llm
        self.model = model

    def ask(self, question: str) -> AgentResult:
        result = AgentResult(question=question, answer="", model=self.model)
        start = time.perf_counter()
        messages = [{"role": "user", "content": question}]
        for _ in range(MAX_ITERATIONS):
            resp = self.llm.messages.create(
                model=self.model,
                max_tokens=2048,
                system=SYSTEM,
                tools=[TOOL_SPEC],
                messages=messages,
            )
            u = getattr(resp, "usage", None)
            if u:
                result.input_tokens += getattr(u, "input_tokens", 0) or 0
                result.output_tokens += getattr(u, "output_tokens", 0) or 0
            blocks = list(resp.content)
            tool_uses = [b for b in blocks if getattr(b, "type", None) == "tool_use"]
            if not tool_uses:
                result.answer = "\n".join(getattr(b, "text", "") for b in blocks if getattr(b, "type", None) == "text").strip()
                break
            messages.append({"role": "assistant", "content": _to_content(blocks)})
            tool_results = []
            for tu in tool_uses:
                args = dict(getattr(tu, "input", {}) or {})
                result.tool_calls.append({"name": tu.name, "input": args})
                text = self._run_tool(tu.name, args)
                tool_results.append({"type": "tool_result", "tool_use_id": tu.id, "content": text})
            messages.append({"role": "user", "content": tool_results})
        else:
            result.answer = "(Stopped: too many tool calls without a final answer.)"
        result.latency_s = time.perf_counter() - start
        result.usdc_spent = self.buyer.guard.spent
        return result

    def _run_tool(self, name: str, args: dict) -> str:
        if name != TOOL_NAME:
            return json.dumps({"error": f"unknown tool {name}"})
        try:
            out = self.buyer.buy(args.get("route", ""), args.get("params") or {})
        except (SpendRefused, KeyError, RuntimeError) as e:
            out = {"error": str(e), "paid": False, "refused": True}
        return json.dumps(out)


def _to_content(blocks) -> list:
    out = []
    for b in blocks:
        if getattr(b, "type", None) == "text":
            out.append({"type": "text", "text": b.text})
        elif getattr(b, "type", None) == "tool_use":
            out.append({"type": "tool_use", "id": b.id, "name": b.name, "input": b.input})
    return out


def x402_http(base_url: str):
    """Live x402 httpx client. Imported only when a real wallet is configured."""
    from eth_account import Account
    from x402 import x402Client
    from x402.http.clients import x402HttpxClient
    from x402.mechanisms.evm import EthAccountSigner
    from x402.mechanisms.evm.exact.register import register_exact_evm_client

    key = os.environ["EVM_PRIVATE_KEY"]
    client = x402Client()
    register_exact_evm_client(client, EthAccountSigner(Account.from_key(key)))
    http = x402HttpxClient(client)

    class Adapter:
        def get(self, url: str) -> tuple[int, str, dict[str, str]]:
            resp = _run(http.get(url))
            headers = {k: v for k, v in resp.headers.items()}
            return resp.status_code, resp.text, headers

    return Adapter()


def _run(coro_or_resp):
    import asyncio
    import inspect

    if inspect.iscoroutine(coro_or_resp):
        return asyncio.get_event_loop().run_until_complete(coro_or_resp)
    return coro_or_resp


def build_from_env() -> ValuationAgent:
    import anthropic

    base = os.environ.get("API_BASE_URL", "http://localhost:8080")
    host = urlparse(base).netloc
    guard = SpendGuard(
        allowed_host=host,
        max_price=parse_price(os.environ.get("MAX_PRICE_PER_CALL", "0.01")),
        question_budget=parse_price(os.environ.get("QUESTION_BUDGET", "0.03")),
    )
    buyer = ValuationBuyer(base, guard, x402_http(base), catalog_http=_plain_catalog(base))
    model = os.environ.get("ANTHROPIC_MODEL", "claude-haiku-4-5")
    return ValuationAgent(buyer, anthropic.Anthropic(), model)


def _plain_catalog(base: str):
    import httpx

    class Plain:
        def get(self, url: str) -> tuple[int, str, dict[str, str]]:
            r = httpx.get(url, timeout=30)
            return r.status_code, r.text, dict(r.headers)

    return Plain()


def main(argv=None) -> None:
    p = argparse.ArgumentParser(description="Ask the valuation API via x402 (testnet).")
    p.add_argument("question", nargs="+")
    args = p.parse_args(argv)
    agent = build_from_env()
    r = agent.ask(" ".join(args.question))
    print(r.answer)
    print(
        f"\n[tokens in {r.input_tokens} / out {r.output_tokens} | LLM ${r.llm_cost_usd:.4f} | "
        f"USDC ${r.usdc_spent:.4f} | {r.latency_s:.1f}s | {len(r.tool_calls)} calls]",
        file=sys.stderr,
    )


if __name__ == "__main__":
    main()
