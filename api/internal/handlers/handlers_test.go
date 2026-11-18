package handlers_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	x402 "github.com/x402-foundation/x402/go/v2"

	"github.com/kevas1010/x402-valuation/api/internal/catalog"
	"github.com/kevas1010/x402-valuation/api/internal/config"
	"github.com/kevas1010/x402-valuation/api/internal/handlers"
	"github.com/kevas1010/x402-valuation/api/internal/server"
	"github.com/kevas1010/x402-valuation/api/internal/store"
	"github.com/kevas1010/x402-valuation/api/internal/x402mw"
)

func init() { gin.SetMode(gin.TestMode) }

func f64(v float64) *float64 { return &v }

func seed(t *testing.T) *store.Memory {
	t.Helper()
	m := store.NewMemory()
	asOf := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	notes, _ := json.Marshal([]map[string]string{{"category": "Info", "item": "Price", "message": "fixture"}})
	detail, _ := json.Marshal(map[string]any{"terminal_value": 100.0, "wacc": 0.12})
	comps, _ := json.Marshal(map[string]any{
		"peers": []any{},
		"medians": map[string]any{"EV/Revenue": 3.0},
		"excluded_outliers": []any{},
	})
	dir := t.TempDir()
	wb := filepath.Join(dir, "AEHR_valuation_2026-10-14.xlsx")
	if err := os.WriteFile(wb, []byte("xlsx"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.Seed(store.Valuation{
		Ticker: "AEHR", AsOf: asOf,
		CurrentPrice: f64(20), DCFValuePerShare: f64(10), CompsValuePerShare: f64(12),
		ProbabilityWeightedPriceTarget: f64(11), BearTarget: f64(6), BaseTarget: f64(10), BullTarget: f64(16),
		WACC: f64(0.12), Beta: f64(1.4), BetaRSquared: f64(0.3), BetaCILow: f64(1.1), BetaCIHigh: f64(1.7),
		TerminalGrowth: f64(0.03), RevenueCAGR: f64(0.2), PeerTickers: []string{"TER", "COHU"},
		DataNotes: notes, DCFDetail: detail, Comps: comps,
	}, []store.SensitivityCell{{WACC: 0.12, TerminalGrowth: 0.03, ValuePerShare: f64(10)}}, wb)
	m.Seed(store.Valuation{
		Ticker: "TER", AsOf: asOf, CurrentPrice: f64(100), DCFValuePerShare: f64(90),
		CompsValuePerShare: f64(95), ProbabilityWeightedPriceTarget: f64(92),
		WACC: f64(0.09), DataNotes: json.RawMessage("[]"), DCFDetail: json.RawMessage("{}"), Comps: json.RawMessage("{}"),
	}, nil, "")
	return m
}

func cfg() config.Config {
	return config.Config{
		PayTo:          "0x1111111111111111111111111111111111111111",
		FacilitatorURL: "https://x402.org/facilitator",
		Network:        catalog.Network,
		ListenAddr:     ":0",
		DatabaseURL:    "unused",
	}
}

func do(r http.Handler, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	r.ServeHTTP(w, req)
	return w
}

func TestFreeEndpoints(t *testing.T) {
	r := server.New(cfg(), seed(t), server.Options{})
	for _, path := range []string{"/health", "/v1/catalog"} {
		w := do(r, "GET", path)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if w.Header().Get("X-Request-ID") == "" {
			t.Fatalf("%s missing request id", path)
		}
	}
	var cat map[string]any
	if err := json.Unmarshal(do(r, "GET", "/v1/catalog").Body.Bytes(), &cat); err != nil {
		t.Fatal(err)
	}
	routes, _ := cat["routes"].([]any)
	if len(routes) != 5 {
		t.Fatalf("catalog routes: %v", routes)
	}
	tickers, _ := cat["covered_tickers"].([]any)
	if len(tickers) != 2 {
		t.Fatalf("tickers %v", tickers)
	}
}

func TestHandlerTables(t *testing.T) {
	r := server.New(cfg(), seed(t), server.Options{})
	tests := []struct {
		name string
		path string
		code int
		want string
	}{
		{"summary", "/v1/valuations/AEHR", 200, `"ticker":"AEHR"`},
		{"dcf", "/v1/valuations/AEHR/dcf", 200, `"wacc":0.12`},
		{"comps", "/v1/valuations/AEHR/comps", 200, `"peer_tickers"`},
		{"compare", "/v1/compare?tickers=AEHR,TER", 200, `"comparisons"`},
		{"unknown", "/v1/valuations/ZZZZ", 404, "unknown ticker"},
		{"bad ticker", "/v1/valuations/aehr!", 400, "invalid ticker"},
		{"too many", "/v1/compare?tickers=A,B,C,D,E,F", 400, "at most 5"},
		{"compare missing", "/v1/compare", 400, "required"},
		{"disclaimer", "/v1/valuations/AEHR", 200, catalog.Disclaimer},
		{"as_of", "/v1/valuations/AEHR", 200, "2026-10-14"},
		{"notes", "/v1/valuations/AEHR", 200, "fixture"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := do(r, "GET", tc.path)
			if w.Code != tc.code {
				t.Fatalf("code %d want %d body %s", w.Code, tc.code, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("body %s missing %q", w.Body.String(), tc.want)
			}
		})
	}
}

func TestWorkbookDownload(t *testing.T) {
	r := server.New(cfg(), seed(t), server.Options{})
	w := do(r, "GET", "/v1/workbooks/AEHR")
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Content-Disposition"), "AEHR_valuation_2026-10-14.xlsx") {
		t.Fatalf("disposition %s", w.Header().Get("Content-Disposition"))
	}
	if w.Header().Get("X-As-Of") != "2026-10-14" {
		t.Fatalf("as_of header %s", w.Header().Get("X-As-Of"))
	}
}

func TestPaymentLogging(t *testing.T) {
	mem := seed(t)
	h := handlers.New(mem, cfg().PayTo, nil)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/v1/valuations/AEHR", nil)
	h.OnSettlement(c, &x402.SettleResponse{
		Success:     true,
		Transaction: "0xabc",
		Network:     x402.Network(catalog.Network),
		Payer:       "0xpayer",
	})
	got, err := mem.ListPayments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 payment, got %#v", got)
	}
	if got[0].SettlementTxHash != "0xabc" || got[0].PayerAddress != "0xpayer" {
		t.Fatalf("%+v", got[0])
	}
	if got[0].Amount != catalog.PriceSummary {
		t.Fatalf("amount %s", got[0].Amount)
	}
	if strings.Contains(got[0].Route, "PAYMENT") {
		t.Fatal("must not log payment headers")
	}
}

func TestUnpaidPaidRouteReturns402(t *testing.T) {
	cfgv := cfg()
	mem := seed(t)
	r := gin.New()
	h := newLocalHandlers(mem, cfgv.PayTo)
	r.Use(func(c *gin.Context) {
		c.Set("request_id", "test")
		c.Header("X-Request-ID", "test")
		c.Next()
	})
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	r.GET("/v1/catalog", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	paid := r.Group("/")
	paid.Use(h.ValidateParams())
	paid.Use(x402mw.Middleware(cfgv, h, false))
	paid.GET("/v1/valuations/:ticker", h.Summary)
	paid.GET("/v1/valuations/:ticker/dcf", h.DCF)
	paid.GET("/v1/compare", h.Compare)

	w := do(r, "GET", "/health")
	if w.Code != 200 {
		t.Fatalf("health paid? %d", w.Code)
	}
	w = do(r, "GET", "/v1/catalog")
	if w.Code != 200 {
		t.Fatalf("catalog paid? %d", w.Code)
	}

	cases := []struct {
		path   string
		amount string
	}{
		{"/v1/valuations/AEHR", "2000"},
		{"/v1/valuations/AEHR/dcf", "3000"},
		{"/v1/compare?tickers=AEHR,TER", "5000"},
	}
	for _, tc := range cases {
		w := do(r, "GET", tc.path)
		if w.Code != http.StatusPaymentRequired {
			t.Fatalf("%s: want 402 got %d %s", tc.path, w.Code, w.Body.String())
		}
		raw := w.Header().Get("Payment-Required")
		if raw == "" {
			raw = w.Header().Get("PAYMENT-REQUIRED")
		}
		decoded, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			t.Fatalf("PAYMENT-REQUIRED: %v", err)
		}
		var req map[string]any
		if err := json.Unmarshal(decoded, &req); err != nil {
			t.Fatalf("decode 402: %v %s", err, decoded)
		}
		accepts, _ := req["accepts"].([]any)
		if len(accepts) == 0 {
			t.Fatalf("no accepts: %s", decoded)
		}
		opt, _ := accepts[0].(map[string]any)
		if opt["network"] != catalog.Network {
			t.Fatalf("network %v", opt["network"])
		}
		if !strings.EqualFold(fmtString(opt["payTo"]), cfgv.PayTo) {
			t.Fatalf("payTo %v", opt["payTo"])
		}
		if opt["amount"] != tc.amount {
			t.Fatalf("%s amount %v want %s", tc.path, opt["amount"], tc.amount)
		}
	}
}

func fmtString(v any) string {
	s, _ := v.(string)
	return s
}
