package handlers_test

import (
	"github.com/kevas1010/x402-valuation/api/internal/handlers"
	"github.com/kevas1010/x402-valuation/api/internal/store"
)

func newLocalHandlers(s store.Store, payTo string) *handlers.Handlers {
	return handlers.New(s, payTo, nil)
}
