package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Valuation struct {
	Ticker                            string
	AsOf                              time.Time
	CurrentPrice                      *float64
	DCFValuePerShare                  *float64
	CompsValuePerShare                *float64
	ProbabilityWeightedPriceTarget    *float64
	BearTarget                        *float64
	BaseTarget                        *float64
	BullTarget                        *float64
	WACC                              *float64
	Beta                              *float64
	BetaRSquared                      *float64
	BetaCILow                         *float64
	BetaCIHigh                        *float64
	TerminalGrowth                    *float64
	RevenueCAGR                       *float64
	PeerTickers                       []string
	DataNotes                         json.RawMessage
	DCFDetail                         json.RawMessage
	Comps                             json.RawMessage
}

type SensitivityCell struct {
	WACC            float64  `json:"wacc"`
	TerminalGrowth  float64  `json:"terminal_growth"`
	ValuePerShare   *float64 `json:"value_per_share"`
}

type Payment struct {
	Route             string
	PayerAddress      string
	Amount            string
	Network           string
	SettlementTxHash  string
}

type Store interface {
	CoveredTickers(ctx context.Context) ([]string, error)
	HasTicker(ctx context.Context, ticker string) (bool, error)
	LatestValuation(ctx context.Context, ticker string) (*Valuation, error)
	Sensitivity(ctx context.Context, ticker string, asOf time.Time) ([]SensitivityCell, error)
	WorkbookPath(ctx context.Context, ticker string) (string, error)
	InsertPayment(ctx context.Context, p Payment) error
	ListPayments(ctx context.Context) ([]Payment, error)
	Close()
}

type PG struct {
	pool *pgxpool.Pool
}

func Connect(ctx context.Context, dsn string) (*PG, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return &PG{pool: pool}, nil
}

func (s *PG) Close() { s.pool.Close() }

func (s *PG) CoveredTickers(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ticker FROM valuations ORDER BY ticker`)
	if err != nil {
		return nil, fmt.Errorf("list tickers: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("scan ticker: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *PG) HasTicker(ctx context.Context, ticker string) (bool, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM valuations WHERE ticker = $1`, ticker).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("has ticker: %w", err)
	}
	return n > 0, nil
}

func (s *PG) LatestValuation(ctx context.Context, ticker string) (*Valuation, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT ticker, as_of, current_price, dcf_value_per_share, comps_value_per_share,
		       probability_weighted_price_target, bear_target, base_target, bull_target,
		       wacc, beta, beta_r_squared, beta_ci_low, beta_ci_high, terminal_growth,
		       revenue_cagr, peer_tickers, data_notes, dcf_detail, comps
		FROM valuations WHERE ticker = $1
		ORDER BY as_of DESC LIMIT 1`, ticker)
	v, err := scanValuation(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("latest valuation %s: %w", ticker, err)
	}
	return v, nil
}

func scanValuation(row pgx.Row) (*Valuation, error) {
	var v Valuation
	err := row.Scan(
		&v.Ticker, &v.AsOf, &v.CurrentPrice, &v.DCFValuePerShare, &v.CompsValuePerShare,
		&v.ProbabilityWeightedPriceTarget, &v.BearTarget, &v.BaseTarget, &v.BullTarget,
		&v.WACC, &v.Beta, &v.BetaRSquared, &v.BetaCILow, &v.BetaCIHigh, &v.TerminalGrowth,
		&v.RevenueCAGR, &v.PeerTickers, &v.DataNotes, &v.DCFDetail, &v.Comps,
	)
	if err != nil {
		return nil, err
	}
	if v.DataNotes == nil {
		v.DataNotes = json.RawMessage("[]")
	}
	if v.DCFDetail == nil {
		v.DCFDetail = json.RawMessage("{}")
	}
	if v.Comps == nil {
		v.Comps = json.RawMessage("{}")
	}
	return &v, nil
}

func (s *PG) Sensitivity(ctx context.Context, ticker string, asOf time.Time) ([]SensitivityCell, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT wacc, terminal_growth, value_per_share
		FROM sensitivity WHERE ticker = $1 AND as_of = $2
		ORDER BY wacc, terminal_growth`, ticker, asOf)
	if err != nil {
		return nil, fmt.Errorf("sensitivity: %w", err)
	}
	defer rows.Close()
	var out []SensitivityCell
	for rows.Next() {
		var c SensitivityCell
		if err := rows.Scan(&c.WACC, &c.TerminalGrowth, &c.ValuePerShare); err != nil {
			return nil, fmt.Errorf("scan sensitivity: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *PG) WorkbookPath(ctx context.Context, ticker string) (string, error) {
	var path string
	err := s.pool.QueryRow(ctx, `
		SELECT path FROM workbooks WHERE ticker = $1 ORDER BY as_of DESC LIMIT 1`, ticker).Scan(&path)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("workbook path: %w", err)
	}
	return path, nil
}

func (s *PG) InsertPayment(ctx context.Context, p Payment) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO payments (route, payer_address, amount, network, settlement_tx_hash)
		VALUES ($1, $2, $3, $4, $5)`,
		p.Route, p.PayerAddress, p.Amount, p.Network, p.SettlementTxHash)
	if err != nil {
		return fmt.Errorf("insert payment: %w", err)
	}
	return nil
}

func (s *PG) ListPayments(ctx context.Context) ([]Payment, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT route, payer_address, amount, network, settlement_tx_hash
		FROM payments ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list payments: %w", err)
	}
	defer rows.Close()
	var out []Payment
	for rows.Next() {
		var p Payment
		if err := rows.Scan(&p.Route, &p.PayerAddress, &p.Amount, &p.Network, &p.SettlementTxHash); err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
