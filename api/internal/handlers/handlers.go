package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	x402 "github.com/x402-foundation/x402/go/v2"

	"github.com/kevas1010/x402-valuation/api/internal/catalog"
	"github.com/kevas1010/x402-valuation/api/internal/store"
)

var tickerRE = regexp.MustCompile(`^[A-Z]{1,10}$`)

type Handlers struct {
	Store  store.Store
	PayTo  string
	Logger *slog.Logger
}

func New(s store.Store, payTo string, log *slog.Logger) *Handlers {
	if log == nil {
		log = slog.Default()
	}
	return &Handlers{Store: s, PayTo: payTo, Logger: log}
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader("X-Request-ID")
		if id == "" {
			id = c.GetHeader("X-Request-Id")
		}
		if id == "" {
			id = randomID()
		}
		c.Set("request_id", id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func (h *Handlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "request_id": reqID(c)})
}

func (h *Handlers) Catalog(c *gin.Context) {
	tickers, err := h.Store.CoveredTickers(c.Request.Context())
	if err != nil {
		h.fail(c, http.StatusInternalServerError, "catalog")
		return
	}
	if tickers == nil {
		tickers = []string{}
	}
	h.ok(c, gin.H{
		"routes":           catalog.PaidRoutes(),
		"covered_tickers":  tickers,
		"network":          catalog.Network,
		"pay_to":           h.PayTo,
		"facilitator_note": "Base Sepolia testnet USDC via x402",
	})
}

func (h *Handlers) ValidateParams() gin.HandlerFunc {
	return func(c *gin.Context) {
		if t := c.Param("ticker"); t != "" {
			sym, errStatus, msg := h.normalizeTicker(c, t)
			if errStatus != 0 {
				h.err(c, errStatus, msg)
				c.Abort()
				return
			}
			c.Set("ticker", sym)
		}
		if raw, ok := c.GetQuery("tickers"); ok || strings.HasPrefix(c.Request.URL.Path, "/v1/compare") {
			list, errStatus, msg := h.parseCompare(c, c.Query("tickers"))
			if errStatus != 0 {
				h.err(c, errStatus, msg)
				c.Abort()
				return
			}
			_ = raw
			c.Set("tickers", list)
		}
		c.Next()
	}
}

func (h *Handlers) normalizeTicker(c *gin.Context, raw string) (string, int, string) {
	sym := strings.ToUpper(strings.TrimSpace(raw))
	if !tickerRE.MatchString(sym) {
		return "", http.StatusBadRequest, "invalid ticker"
	}
	ok, err := h.Store.HasTicker(c.Request.Context(), sym)
	if err != nil {
		h.Logger.Error("has ticker", "err", err, "request_id", reqID(c))
		return "", http.StatusInternalServerError, "internal error"
	}
	if !ok {
		return "", http.StatusNotFound, "unknown ticker"
	}
	return sym, 0, ""
}

func (h *Handlers) parseCompare(c *gin.Context, raw string) ([]string, int, string) {
	if strings.TrimSpace(raw) == "" {
		return nil, http.StatusBadRequest, "tickers query param is required"
	}
	parts := strings.Split(raw, ",")
	var list []string
	seen := map[string]bool{}
	for _, p := range parts {
		sym := strings.ToUpper(strings.TrimSpace(p))
		if sym == "" {
			continue
		}
		if !tickerRE.MatchString(sym) {
			return nil, http.StatusBadRequest, "invalid ticker"
		}
		if seen[sym] {
			continue
		}
		seen[sym] = true
		list = append(list, sym)
	}
	if len(list) == 0 {
		return nil, http.StatusBadRequest, "tickers query param is required"
	}
	if len(list) > 5 {
		return nil, http.StatusBadRequest, "compare supports at most 5 tickers"
	}
	for _, sym := range list {
		ok, err := h.Store.HasTicker(c.Request.Context(), sym)
		if err != nil {
			h.Logger.Error("has ticker", "err", err, "request_id", reqID(c))
			return nil, http.StatusInternalServerError, "internal error"
		}
		if !ok {
			return nil, http.StatusNotFound, "unknown ticker"
		}
	}
	return list, 0, ""
}

func (h *Handlers) Summary(c *gin.Context) {
	v, ok := h.load(c)
	if !ok {
		return
	}
	h.ok(c, gin.H{
		"ticker":           v.Ticker,
		"as_of":            v.AsOf.Format("2006-01-02"),
		"current_price":    v.CurrentPrice,
		"dcf_value":        v.DCFValuePerShare,
		"comps_value":      v.CompsValuePerShare,
		"price_target":     v.ProbabilityWeightedPriceTarget,
		"upside_pct":       upside(v),
		"data_notes":       json.RawMessage(v.DataNotes),
	})
}

func (h *Handlers) DCF(c *gin.Context) {
	v, ok := h.load(c)
	if !ok {
		return
	}
	grid, err := h.Store.Sensitivity(c.Request.Context(), v.Ticker, v.AsOf)
	if err != nil {
		h.fail(c, http.StatusInternalServerError, "sensitivity")
		return
	}
	h.ok(c, gin.H{
		"ticker":           v.Ticker,
		"as_of":            v.AsOf.Format("2006-01-02"),
		"wacc":             v.WACC,
		"beta":             v.Beta,
		"beta_r_squared":   v.BetaRSquared,
		"beta_ci_95":       gin.H{"low": v.BetaCILow, "high": v.BetaCIHigh},
		"revenue_cagr":     v.RevenueCAGR,
		"terminal_growth":  v.TerminalGrowth,
		"dcf_detail":       json.RawMessage(v.DCFDetail),
		"sensitivity":      grid,
		"data_notes":       json.RawMessage(v.DataNotes),
	})
}

func (h *Handlers) Comps(c *gin.Context) {
	v, ok := h.load(c)
	if !ok {
		return
	}
	h.ok(c, gin.H{
		"ticker":      v.Ticker,
		"as_of":       v.AsOf.Format("2006-01-02"),
		"peer_tickers": v.PeerTickers,
		"comps":       json.RawMessage(v.Comps),
		"data_notes":  json.RawMessage(v.DataNotes),
	})
}

func (h *Handlers) Compare(c *gin.Context) {
	raw, _ := c.Get("tickers")
	list, _ := raw.([]string)
	var rows []gin.H
	for _, t := range list {
		v, err := h.Store.LatestValuation(c.Request.Context(), t)
		if err != nil {
			h.fail(c, http.StatusInternalServerError, "valuation")
			return
		}
		if v == nil {
			h.err(c, http.StatusNotFound, "unknown ticker")
			return
		}
		rows = append(rows, gin.H{
			"ticker":        v.Ticker,
			"as_of":         v.AsOf.Format("2006-01-02"),
			"current_price": v.CurrentPrice,
			"dcf_value":     v.DCFValuePerShare,
			"comps_value":   v.CompsValuePerShare,
			"price_target":  v.ProbabilityWeightedPriceTarget,
			"upside_pct":    upside(v),
			"data_notes":    json.RawMessage(v.DataNotes),
		})
	}
	h.ok(c, gin.H{"comparisons": rows})
}

func (h *Handlers) Workbook(c *gin.Context) {
	ticker := tickerFrom(c)
	path, err := h.Store.WorkbookPath(c.Request.Context(), ticker)
	if err != nil {
		h.fail(c, http.StatusInternalServerError, "workbook")
		return
	}
	if path == "" {
		h.err(c, http.StatusNotFound, "workbook not found")
		return
	}
	if _, err := os.Stat(path); err != nil {
		h.err(c, http.StatusNotFound, "workbook file missing")
		return
	}
	v, err := h.Store.LatestValuation(c.Request.Context(), ticker)
	if err != nil || v == nil {
		h.fail(c, http.StatusInternalServerError, "valuation")
		return
	}
	c.Header("X-As-Of", v.AsOf.Format("2006-01-02"))
	c.Header("X-Disclaimer", catalog.Disclaimer)
	c.Header("X-Request-ID", reqID(c))
	c.FileAttachment(path, filepath.Base(path))
}

func (h *Handlers) OnSettlement(c *gin.Context, s *x402.SettleResponse) {
	if s == nil {
		return
	}
	p := store.Payment{
		Route:            c.Request.Method + " " + c.Request.URL.Path,
		PayerAddress:     s.Payer,
		Amount:           catalog.PriceForPath(c.Request.URL.Path),
		Network:          string(s.Network),
		SettlementTxHash: s.Transaction,
	}
	if p.Network == "" {
		p.Network = catalog.Network
	}
	if err := h.Store.InsertPayment(c.Request.Context(), p); err != nil {
		h.Logger.Error("payment log failed", "err", err, "request_id", reqID(c), "route", p.Route)
		return
	}
	h.Logger.Info("payment settled",
		"route", p.Route,
		"payer", p.PayerAddress,
		"amount", p.Amount,
		"network", p.Network,
		"tx", p.SettlementTxHash,
		"request_id", reqID(c),
	)
}

func (h *Handlers) load(c *gin.Context) (*store.Valuation, bool) {
	v, err := h.Store.LatestValuation(c.Request.Context(), tickerFrom(c))
	if err != nil {
		h.fail(c, http.StatusInternalServerError, "valuation")
		return nil, false
	}
	if v == nil {
		h.err(c, http.StatusNotFound, "unknown ticker")
		return nil, false
	}
	return v, true
}

func tickerFrom(c *gin.Context) string {
	if v, ok := c.Get("ticker"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return strings.ToUpper(c.Param("ticker"))
}

func upside(v *store.Valuation) *float64 {
	if v.ProbabilityWeightedPriceTarget == nil || v.CurrentPrice == nil || *v.CurrentPrice == 0 {
		return nil
	}
	u := *v.ProbabilityWeightedPriceTarget / *v.CurrentPrice - 1
	return &u
}

func (h *Handlers) ok(c *gin.Context, body gin.H) {
	body["disclaimer"] = catalog.Disclaimer
	body["request_id"] = reqID(c)
	c.JSON(http.StatusOK, body)
}

func (h *Handlers) err(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{
		"error":       msg,
		"disclaimer":  catalog.Disclaimer,
		"request_id":  reqID(c),
	})
}

func (h *Handlers) fail(c *gin.Context, status int, op string) {
	h.Logger.Error(op, "request_id", reqID(c))
	h.err(c, status, "internal error")
}

func reqID(c *gin.Context) string {
	if v, ok := c.Get("request_id"); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return c.GetHeader("X-Request-ID")
}
