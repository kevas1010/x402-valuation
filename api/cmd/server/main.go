package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kevas1010/x402-valuation/api/internal/config"
	"github.com/kevas1010/x402-valuation/api/internal/server"
	"github.com/kevas1010/x402-valuation/api/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(log)

	cfg, err := config.FromEnv()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	r := server.New(cfg, db, server.Options{Pay: true, SyncFacilitator: true})

	go func() {
		log.Info("listening", "addr", cfg.ListenAddr, "network", cfg.Network, "facilitator", cfg.FacilitatorURL)
		if err := r.Run(cfg.ListenAddr); err != nil {
			log.Error("server", "err", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Info("shutting down")
}
