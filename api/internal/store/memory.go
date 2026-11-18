package store

import (
	"context"
	"strings"
	"sync"
	"time"
)

// Memory is an in-process store for table-driven handler tests.
type Memory struct {
	mu          sync.Mutex
	valuations  map[string]Valuation
	sensitivity map[string][]SensitivityCell
	workbooks   map[string]string
	payments    []Payment
}

func NewMemory() *Memory {
	return &Memory{
		valuations:  map[string]Valuation{},
		sensitivity: map[string][]SensitivityCell{},
		workbooks:   map[string]string{},
	}
}

func (m *Memory) Seed(v Valuation, grid []SensitivityCell, workbook string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := strings.ToUpper(v.Ticker)
	m.valuations[key] = v
	m.sensitivity[key] = grid
	m.workbooks[key] = workbook
}

func (m *Memory) Close() {}

func (m *Memory) CoveredTickers(ctx context.Context) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.valuations))
	for t := range m.valuations {
		out = append(out, t)
	}
	return out, nil
}

func (m *Memory) HasTicker(ctx context.Context, ticker string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.valuations[strings.ToUpper(ticker)]
	return ok, nil
}

func (m *Memory) LatestValuation(ctx context.Context, ticker string) (*Valuation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.valuations[strings.ToUpper(ticker)]
	if !ok {
		return nil, nil
	}
	cp := v
	return &cp, nil
}

func (m *Memory) Sensitivity(ctx context.Context, ticker string, asOf time.Time) ([]SensitivityCell, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sensitivity[strings.ToUpper(ticker)], nil
}

func (m *Memory) WorkbookPath(ctx context.Context, ticker string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.workbooks[strings.ToUpper(ticker)], nil
}

func (m *Memory) InsertPayment(ctx context.Context, p Payment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.payments = append(m.payments, p)
	return nil
}

func (m *Memory) ListPayments(ctx context.Context) ([]Payment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Payment, len(m.payments))
	copy(out, m.payments)
	return out, nil
}
