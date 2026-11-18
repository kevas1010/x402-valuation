from __future__ import annotations

from dataclasses import dataclass, field
from urllib.parse import urlparse


class SpendRefused(Exception):
    def __init__(self, reason: str):
        super().__init__(reason)
        self.reason = reason


@dataclass
class SpendGuard:
    allowed_host: str
    max_price: float = 0.01
    question_budget: float = 0.03
    spent: float = 0.0
    calls: list[dict] = field(default_factory=list)

    def check(self, url: str, price: float) -> None:
        host = urlparse(url).netloc.lower()
        allowed = self.allowed_host.lower()
        if "://" in allowed:
            allowed = urlparse(self.allowed_host).netloc.lower()
        if host != allowed:
            raise SpendRefused(f"host {host!r} is not the allowlisted API host {allowed!r}")
        if price > self.max_price + 1e-12:
            raise SpendRefused(f"price ${price:.4f} exceeds max per call ${self.max_price:.4f}")
        if self.spent + price > self.question_budget + 1e-12:
            raise SpendRefused(
                f"call would spend ${price:.4f} after ${self.spent:.4f} already spent, "
                f"over the ${self.question_budget:.4f} question budget"
            )

    def record(self, url: str, price: float, route: str) -> None:
        self.spent += price
        self.calls.append({"url": url, "price": price, "route": route})
