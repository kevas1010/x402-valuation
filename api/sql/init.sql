CREATE USER publisher WITH PASSWORD 'publisher';
CREATE USER api WITH PASSWORD 'api';

CREATE TABLE valuations (
    ticker TEXT NOT NULL,
    as_of DATE NOT NULL,
    current_price DOUBLE PRECISION,
    dcf_value_per_share DOUBLE PRECISION,
    comps_value_per_share DOUBLE PRECISION,
    probability_weighted_price_target DOUBLE PRECISION,
    bear_target DOUBLE PRECISION,
    base_target DOUBLE PRECISION,
    bull_target DOUBLE PRECISION,
    wacc DOUBLE PRECISION,
    beta DOUBLE PRECISION,
    beta_r_squared DOUBLE PRECISION,
    beta_ci_low DOUBLE PRECISION,
    beta_ci_high DOUBLE PRECISION,
    terminal_growth DOUBLE PRECISION,
    revenue_cagr DOUBLE PRECISION,
    peer_tickers TEXT[] NOT NULL DEFAULT '{}',
    data_notes JSONB NOT NULL DEFAULT '[]'::jsonb,
    dcf_detail JSONB NOT NULL DEFAULT '{}'::jsonb,
    comps JSONB NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (ticker, as_of)
);

CREATE TABLE sensitivity (
    ticker TEXT NOT NULL,
    as_of DATE NOT NULL,
    wacc DOUBLE PRECISION NOT NULL,
    terminal_growth DOUBLE PRECISION NOT NULL,
    value_per_share DOUBLE PRECISION,
    PRIMARY KEY (ticker, as_of, wacc, terminal_growth)
);

CREATE TABLE workbooks (
    ticker TEXT NOT NULL,
    as_of DATE NOT NULL,
    path TEXT NOT NULL,
    PRIMARY KEY (ticker, as_of)
);

CREATE TABLE payments (
    id BIGSERIAL PRIMARY KEY,
    route TEXT NOT NULL,
    payer_address TEXT NOT NULL,
    amount TEXT NOT NULL,
    network TEXT NOT NULL,
    settlement_tx_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

GRANT SELECT, INSERT, UPDATE, DELETE ON valuations, sensitivity, workbooks TO publisher;
GRANT SELECT ON valuations, sensitivity, workbooks TO api;
GRANT SELECT, INSERT ON payments TO api;
GRANT USAGE, SELECT ON SEQUENCE payments_id_seq TO api;
