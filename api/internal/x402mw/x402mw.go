package x402mw

import (
	"time"

	"github.com/gin-gonic/gin"
	x402 "github.com/x402-foundation/x402/go/v2"
	x402http "github.com/x402-foundation/x402/go/v2/http"
	ginmw "github.com/x402-foundation/x402/go/v2/http/gin"
	evm "github.com/x402-foundation/x402/go/v2/mechanisms/evm/exact/server"

	"github.com/kevas1010/x402-valuation/api/internal/catalog"
	"github.com/kevas1010/x402-valuation/api/internal/config"
	"github.com/kevas1010/x402-valuation/api/internal/handlers"
)

func Routes(payTo, network string) x402http.RoutesConfig {
	opt := func(price, desc string) x402http.RouteConfig {
		return x402http.RouteConfig{
			Accepts: x402http.PaymentOptions{
				{
					Scheme:  "exact",
					Price:   x402.Price(price),
					Network: x402.Network(network),
					PayTo:   payTo,
				},
			},
			Description: desc,
			MimeType:    "application/json",
		}
	}
	return x402http.RoutesConfig{
		"GET /v1/valuations/*/dcf":   opt(catalog.PriceDCF, "DCF detail"),
		"GET /v1/valuations/*/comps": opt(catalog.PriceComps, "Comps detail"),
		"GET /v1/valuations/*":       opt(catalog.PriceSummary, "Valuation summary"),
		"GET /v1/compare":            opt(catalog.PriceCompare, "Side-by-side compare"),
		"GET /v1/workbooks/*":        opt(catalog.PriceWorkbook, "Excel workbook"),
	}
}

func Middleware(cfg config.Config, h *handlers.Handlers, syncFacilitator bool) gin.HandlerFunc {
	facilitator := x402http.NewHTTPFacilitatorClient(&x402http.FacilitatorConfig{URL: cfg.FacilitatorURL})
	return ginmw.PaymentMiddlewareFromConfig(
		Routes(cfg.PayTo, cfg.Network),
		ginmw.WithFacilitatorClient(facilitator),
		ginmw.WithScheme(x402.Network(cfg.Network), evm.NewExactEvmScheme()),
		ginmw.WithSyncFacilitatorOnStart(syncFacilitator),
		ginmw.WithTimeout(30*time.Second),
		ginmw.WithSettlementHandler(h.OnSettlement),
	)
}
