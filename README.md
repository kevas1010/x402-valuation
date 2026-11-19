# x402 Pay-Per-Request Valuation API

A Go API that sells stock valuations from the [EDGAR Equity Valuation Model](https://github.com/kevas1010/edgar-valuation), one request at a time, for **test USDC on Base Sepolia** using [x402](https://www.x402.org/). A small Claude agent discovers the paid endpoints, pays inside a spending cap, and answers questions like “Is TER cheap compared to its peers?”

**Testnet only. Model output, not investment advice.**

## How a paid request works

```
agent  --GET-->  API
                 | 402 Payment Required + price
agent  signs USDC (Base Sepolia)
agent  --GET + payment header-->  API
API    --verify/settle-->  facilitator (https://x402.org/facilitator)
facilitator  --tx-->  Base Sepolia
API    --JSON (as_of, Data Notes, numbers)-->  agent
```

- **Why x402:** pay per request. No API keys, accounts, or monthly plans.
- **Why spending caps:** the agent can only pay this host, only up to `$0.01` per call and `$0.03` per question, enforced in code before any signature.
- **Why fixed endpoints:** parameterized SQL only; the model is not a free-form prompt.
- **Why `as_of` and Data Notes on every response:** the buyer can see how fresh the run is and which filings were missing.

## Layout

| Path | Role |
|---|---|
| `publisher/` | Thin Python wrapper. Runs the EDGAR model for 20 tickers and upserts Postgres. |
| `api/` | Gin + pgx + official `github.com/x402-foundation/x402/go/v2` middleware. |
| `agent/` | Claude tool loop + x402 Python client + spend guardrails. |
| `vendor/edgar-valuation/` | Unmodified EDGAR model. |

Covered tickers (default): AEHR, TER, COHU, FORM, KLIC, AMAT, KLAC, LRCX, MSFT, GOOGL, ORCL, AMZN, CRM, AAPL, NVDA, AMD, INTC, QCOM, AVGO, TXN.

## Endpoints

| Route | Price | Auth |
|---|---|---|
| `GET /health` | free | |
| `GET /v1/catalog` | free | discovery for the agent |
| `GET /v1/valuations/{ticker}` | $0.002 | x402 |
| `GET /v1/valuations/{ticker}/dcf` | $0.003 | x402 |
| `GET /v1/valuations/{ticker}/comps` | $0.003 | x402 |
| `GET /v1/compare?tickers=A,B,C` | $0.005 | x402 (max 5) |
| `GET /v1/workbooks/{ticker}` | $0.01 | x402 (xlsx download) |

Network: Base Sepolia (`eip155:84532`). Facilitator URL is config (`FACILITATOR_URL`), default `https://x402.org/facilitator`.

## Run

```bash
cp .env.example .env          # set PAY_TO to a Base Sepolia address
docker compose up --build     # db, publisher (once), api on :8080
```

Publisher can take a while on a cold EDGAR/yfinance pull. Restrict `TICKERS` in `.env` for a faster first boot (AEHR,TER,MSFT).

Agent (separate process, keys never in the API `.env`):

```bash
cp agent/.env.example agent/.env
# ANTHROPIC_API_KEY, EVM_PRIVATE_KEY (testnet only), fund with CDP Base Sepolia USDC faucet
cd agent
python -m venv .venv && .venv/Scripts/activate
pip install -r requirements.txt
python -m valuation_agent.agent "Is TER cheap compared to its peers?"
```

## Tests

```bash
# Go (from api/)
go test ./...

# Publisher, offline EDGAR fixtures (optional TEST_DATABASE_URL for upsert)
cd publisher && pytest

# Agent guardrails, no real payments
cd agent && pytest
```

## Demo (Base Sepolia)

Verified locally (2026-11-18): API on `:8080`, Postgres seeded with **AEHR** and **TER** from offline EDGAR fixtures. Unpaid `GET /v1/valuations/AEHR` returns **402** with network `eip155:84532` and $0.002 (2000 USDC units). Live agent payments are blocked until the buyer wallet has Base Sepolia ETH + test USDC.

Merchant (`PAY_TO`): `0x409e1fF90624a7B054E6349AC61a75d5028488C8`  
Buyer (agent): `0x8eb73A6Ae8d33F745B160b7496c6675b02369430`

Fund the **buyer** from the [CDP Base Sepolia faucet](https://portal.cdp.coinbase.com/products/faucet) (ETH and USDC), then:

```bash
python scripts/run_demo.py
```

| Question | Endpoints | USDC | Settlement tx | Answer |
|---|---|---|---|---|
| Compare TER and AEHR on comps value | | | | |
| What WACC does the model use for AEHR, and why is the DCF so far below the price? | | | | |
| Is TER cheap vs AEHR on the probability-weighted target? | | | | |

BaseScan: `https://sepolia.basescan.org/tx/<hash>`
