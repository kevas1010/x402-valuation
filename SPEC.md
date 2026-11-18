# x402 Pay-Per-Request Valuation API

## What it is

A Go API that sells stock valuations from the EDGAR Equity Valuation Model, one request at a time,
for test USDC on Base Sepolia using x402. A small LLM agent finds the paid endpoints, pays for what
it needs (inside a spending cap), and answers questions like "Is TER cheap compared to its peers?"

How x402 works in one line: an unpaid request gets HTTP **402 Payment Required** with the price; the
client signs a USDC payment, retries with a payment header, a facilitator verifies and settles it on
Base Sepolia, and the API returns the data.

Three pieces, nothing else:
1. `publisher/`: a thin Python wrapper around the existing EDGAR model that writes results to
   Postgres.
2. `api/`: Go server (Gin) + PostgreSQL + Docker Compose.
3. `agent/`: a small Python agent with one paid-data tool.

## 1. Publisher (`publisher/`)

The EDGAR model stays as it is. Add one command that runs it for a list of tickers and writes the
results to Postgres:

- `valuations` table: ticker, as_of date, current price, DCF value per share, comps value per share,
  probability-weighted price target, bear/base/bull targets, WACC, beta (with R² and 95% CI), terminal
  growth, revenue CAGR used, peer tickers, and the Data Notes (missing-data warnings) as JSON.
- `sensitivity` table: the WACC × terminal-growth grid per ticker.
- `workbooks` table or a folder: path to the generated `.xlsx` per ticker.
- Start with 10 to 20 tickers (include the ones already tested, like AEHR, TER and MSFT).
- Re-running for a ticker replaces that day's row (upsert on ticker + as_of).

## 2. Go API (`api/`)

Stack: Go, Gin, the official x402 Go module (`github.com/x402-foundation/x402/go/v2`), pgx, Docker Compose.

x402 setup:
- Network: Base Sepolia, CAIP-2 id `eip155:84532`.
- Facilitator: the public testnet facilitator `https://x402.org/facilitator` (no API key, Base Sepolia
  only). Keep the URL in config so it can switch to Coinbase's CDP facilitator later.
- `PAY_TO`: testnet receiving address (env var).

Endpoints:
| Route | Price | Returns |
|---|---|---|
| `GET /health` | free | |
| `GET /v1/catalog` | free | Every paid route, its price, params, a one-line description, and the list of covered tickers. This is how the agent "discovers" the API. |
| `GET /v1/valuations/{ticker}` | $0.002 | Summary: price, DCF value, comps value, price target, upside %, as_of |
| `GET /v1/valuations/{ticker}/dcf` | $0.003 | WACC, beta stats, growth, terminal value, sensitivity grid |
| `GET /v1/valuations/{ticker}/comps` | $0.003 | Peer multiples (EV/Revenue, EV/EBITDA, P/E), medians, excluded outliers |
| `GET /v1/compare?tickers=A,B,C` | $0.005 | Side-by-side summary, max 5 tickers |
| `GET /v1/workbooks/{ticker}` | $0.01 | Downloads the live-formula Excel model |

Rules:
- Fixed, parameterized queries only. Validate tickers against the covered list; 404 for unknown,
  400 for bad params.
- Every response includes `as_of` and the Data Notes, so a buyer knows how fresh and complete it is.
- `payments` table: route, payer address, amount, network, settlement tx hash, timestamp. Log after
  each successful settlement. Never log full payment headers.
- Structured JSON logs, a request ID on every response, read-only DB user for the API (SELECT on
  valuation tables; INSERT on `payments`).
- Response footer: "Model output, not investment advice. Testnet only."

## 3. Agent (`agent/`)

Stack: Python, Anthropic Claude API with tool calling (same hand-written loop style as the NBA
agent), the x402 Python client.

- One tool, `buy_valuation(route, params)`:
  - Fetches `/v1/catalog` once and caches it.
  - Calls the route with the x402 client, which handles the 402 → sign → retry flow.
- Spending guardrails, enforced in code before any payment:
  - Only pays the configured API host.
  - Max price per call (e.g. $0.01) and a per-question budget (e.g. $0.03).
  - Refuses and tells the model why when a call would break a limit.
- The agent must cite the as_of date and mention any Data Notes in its answer.
- Telemetry per question: tokens, LLM cost, USDC spent, latency.
- Wallet: a testnet-only key funded with test USDC from the CDP faucet. Never a real wallet.

## 4. Tests

Go (`go test ./...`, table-driven):
- Unpaid request to a paid route returns 402 with the right price, network and pay-to.
- `/health` and `/v1/catalog` are free.
- Unknown ticker 404; more than 5 tickers in compare 400.
- Handlers return correct data against a seeded test Postgres (Compose or testcontainers).
- Payment logging writes one row per settled request.

Python (pytest, fake HTTP and LLM clients, no real payments):
- Publisher upserts and writes the sensitivity grid correctly for a captured EDGAR run (reuse
  existing offline fixtures).
- Over-budget, over-max-price and non-allowlisted-host calls are refused before signing.
- Spend telemetry adds up across a multi-call question.

Real payments are tested by hand on Base Sepolia (section 6).

## 5. Docker Compose

`docker compose up` starts `db` (Postgres), `publisher` (runs once for the ticker list) and `api`.
`.env` holds `PAY_TO`, `FACILITATOR_URL`, `DATABASE_URL`. The agent's private key and Anthropic key
live only in `agent/.env`. All `.env` files are gitignored.

## 6. Demo and README

Run 3 to 5 real questions through the agent on Base Sepolia. Record for each: endpoints called,
USDC paid, settlement tx links on BaseScan, and the answer. Put that table in the README with:
- A diagram: agent → 402 → pay → facilitator → Base Sepolia → valuation data.
- Why x402 (pay per request, no API keys or accounts), why spending caps, why fixed endpoints, why
  every response carries as_of and Data Notes.
- How to run it, test commands, "testnet only, not investment advice."

## 7. Three days

- **Day 1:** publisher writing 10 to 20 tickers to Postgres; Go + Gin skeleton with free endpoints and
  handlers, with tests, in Compose.
- **Day 2:** x402 middleware on paid routes, 402 tests, payment logging; first real paid request on
  Base Sepolia.
- **Day 3:** agent with spending caps and tests; 3 to 5 real questions; README and demo video.

## 8. Cursor rules (`.cursor/rules/x402-valuation.mdc`)

- Follow SPEC.md. Don't add features that aren't in it.
- Don't change the EDGAR model's math; the publisher only wraps it.
- Go: idiomatic, small packages (handlers, store, config), errors wrapped with context, table-driven tests.
- Use the official x402 Go module and its Gin middleware. Check the current x402 docs before writing x402 code.
- Parameterized queries only. Every behavior gets a test; run go test / pytest before saying done.
- Secrets only in .env files, never committed. Testnet-only keys.
