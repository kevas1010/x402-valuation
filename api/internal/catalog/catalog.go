package catalog

const (
	Network        = "eip155:84532"
	Disclaimer     = "Model output, not investment advice. Testnet only."
	PriceSummary   = "$0.002"
	PriceDCF       = "$0.003"
	PriceComps     = "$0.003"
	PriceCompare   = "$0.005"
	PriceWorkbook  = "$0.01"
)

type Route struct {
	ID          string            `json:"id"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Price       string            `json:"price"`
	Params      map[string]string `json:"params"`
	Description string            `json:"description"`
}

func PaidRoutes() []Route {
	return []Route{
		{
			ID:     "valuation",
			Method: "GET",
			Path:   "/v1/valuations/{ticker}",
			Price:  PriceSummary,
			Params: map[string]string{"ticker": "Covered ticker symbol, e.g. TER"},
			Description: "Summary: price, DCF value, comps value, price target, upside %, as_of",
		},
		{
			ID:     "dcf",
			Method: "GET",
			Path:   "/v1/valuations/{ticker}/dcf",
			Price:  PriceDCF,
			Params: map[string]string{"ticker": "Covered ticker symbol"},
			Description: "WACC, beta stats, growth, terminal value, sensitivity grid",
		},
		{
			ID:     "comps",
			Method: "GET",
			Path:   "/v1/valuations/{ticker}/comps",
			Price:  PriceComps,
			Params: map[string]string{"ticker": "Covered ticker symbol"},
			Description: "Peer multiples (EV/Revenue, EV/EBITDA, P/E), medians, excluded outliers",
		},
		{
			ID:     "compare",
			Method: "GET",
			Path:   "/v1/compare",
			Price:  PriceCompare,
			Params: map[string]string{"tickers": "Comma-separated tickers, max 5"},
			Description: "Side-by-side summary for up to 5 tickers",
		},
		{
			ID:     "workbook",
			Method: "GET",
			Path:   "/v1/workbooks/{ticker}",
			Price:  PriceWorkbook,
			Params: map[string]string{"ticker": "Covered ticker symbol"},
			Description: "Downloads the live-formula Excel model",
		},
	}
}

func PriceForPath(path string) string {
	switch {
	case matchWorkbook(path):
		return PriceWorkbook
	case hasSuffix(path, "/dcf"):
		return PriceDCF
	case hasSuffix(path, "/comps"):
		return PriceComps
	case hasPrefix(path, "/v1/compare"):
		return PriceCompare
	default:
		return PriceSummary
	}
}

func hasSuffix(s, suf string) bool {
	return len(s) >= len(suf) && s[len(s)-len(suf):] == suf
}

func hasPrefix(s, pre string) bool {
	return len(s) >= len(pre) && s[:len(pre)] == pre
}

func matchWorkbook(path string) bool {
	return hasPrefix(path, "/v1/workbooks/")
}
