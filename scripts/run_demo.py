"""Ask the three README demo questions through the live agent and print a table."""
from __future__ import annotations

import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "agent"))

from dotenv import load_dotenv

load_dotenv(ROOT / "agent" / ".env")
load_dotenv(ROOT / ".env")

from valuation_agent.agent import build_from_env  # noqa: E402

QUESTIONS = [
    "Compare TER and AEHR on comps value. Cite as_of and any Data Notes.",
    "What WACC does the model use for AEHR, and why is the DCF so far below the price? Cite as_of and Data Notes.",
    "Is TER cheap vs AEHR on the probability-weighted target? Cite as_of and Data Notes.",
]


def main() -> None:
    agent = build_from_env()
    rows = []
    for q in QUESTIONS:
        print(f"\n=== {q} ===\n", flush=True)
        # Fresh budget per question
        agent.buyer.guard.spent = 0.0
        agent.buyer.guard.calls = []
        r = agent.ask(q)
        endpoints = [c["input"] for c in r.tool_calls]
        rows.append(
            {
                "question": q,
                "endpoints": endpoints,
                "usdc": r.usdc_spent,
                "answer": r.answer,
                "tokens_in": r.input_tokens,
                "tokens_out": r.output_tokens,
                "llm_usd": r.llm_cost_usd,
                "latency_s": r.latency_s,
            }
        )
        print(r.answer)
        print(f"USDC ${r.usdc_spent:.4f} | LLM ${r.llm_cost_usd:.4f} | {r.latency_s:.1f}s", flush=True)
    out = ROOT / "work" / "demo_results.json"
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(json.dumps(rows, indent=2), encoding="utf-8")
    print(f"\nWrote {out}")


if __name__ == "__main__":
    main()
