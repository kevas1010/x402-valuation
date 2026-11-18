package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	ListenAddr     string
	DatabaseURL    string
	PayTo          string
	FacilitatorURL string
	Network        string
}

func FromEnv() (Config, error) {
	c := Config{
		ListenAddr:     getenv("LISTEN_ADDR", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		PayTo:          os.Getenv("PAY_TO"),
		FacilitatorURL: getenv("FACILITATOR_URL", "https://x402.org/facilitator"),
		Network:        getenv("NETWORK", "eip155:84532"),
	}
	var missing []string
	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if c.PayTo == "" {
		missing = append(missing, "PAY_TO")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}
	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
