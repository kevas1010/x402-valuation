package server

import (
	"github.com/gin-gonic/gin"

	"github.com/kevas1010/x402-valuation/api/internal/config"
	"github.com/kevas1010/x402-valuation/api/internal/handlers"
	"github.com/kevas1010/x402-valuation/api/internal/store"
	"github.com/kevas1010/x402-valuation/api/internal/x402mw"
)

type Options struct {
	Pay                 bool
	SyncFacilitator     bool
}

func New(cfg config.Config, s store.Store, opts Options) *gin.Engine {
	h := handlers.New(s, cfg.PayTo, nil)
	r := gin.New()
	r.Use(gin.Recovery(), handlers.RequestID())
	r.GET("/health", h.Health)
	r.GET("/v1/catalog", h.Catalog)

	paid := r.Group("/")
	paid.Use(h.ValidateParams())
	if opts.Pay {
		paid.Use(x402mw.Middleware(cfg, h, opts.SyncFacilitator))
	}
	paid.GET("/v1/valuations/:ticker", h.Summary)
	paid.GET("/v1/valuations/:ticker/dcf", h.DCF)
	paid.GET("/v1/valuations/:ticker/comps", h.Comps)
	paid.GET("/v1/compare", h.Compare)
	paid.GET("/v1/workbooks/:ticker", h.Workbook)
	return r
}
