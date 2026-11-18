package config

import "testing"

func TestFromEnvMissing(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PAY_TO", "")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected missing env error")
	}
}

func TestFromEnvDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://api:api@localhost:5432/valuations")
	t.Setenv("PAY_TO", "0xabc")
	t.Setenv("FACILITATOR_URL", "")
	t.Setenv("NETWORK", "")
	t.Setenv("LISTEN_ADDR", "")
	c, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if c.FacilitatorURL != "https://x402.org/facilitator" {
		t.Fatalf("facilitator %s", c.FacilitatorURL)
	}
	if c.Network != "eip155:84532" {
		t.Fatalf("network %s", c.Network)
	}
	if c.ListenAddr != ":8080" {
		t.Fatalf("listen %s", c.ListenAddr)
	}
}
