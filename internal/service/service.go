package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/settlegraph/settlegraph/internal/banksim"
	"github.com/settlegraph/settlegraph/internal/config"
	"github.com/settlegraph/settlegraph/internal/health"
	"github.com/settlegraph/settlegraph/internal/httpapi"
	"github.com/settlegraph/settlegraph/internal/jobs"
)

func Run(role config.Role) error {
	cfg, err := config.Load(role)
	if err != nil {
		return err
	}
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(4)

	mux := http.NewServeMux()
	var bank banksim.HTTP
	var productAPI httpapi.API
	mux.Handle("/health/", health.Handler(string(role), db))
	if role == config.API {
		productAPI = httpapi.API{DB: db, Config: cfg}
		mux.Handle("/v1/", productAPI.Handler())
	}
	if role == config.BankSim {
		bank = banksim.HTTP{Ledger: banksim.Ledger{DB: db}, Config: cfg}
		bankHandler := bank.Handler()
		mux.Handle("/provider/", bankHandler)
		mux.Handle("/admin/", bankHandler)
		mux.Handle("/evaluator/", bankHandler)
	}
	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if role == config.Worker {
		go jobs.Loop(ctx, db)
	}
	if role == config.BankSim {
		go bank.DeliverLoop(ctx)
	}
	if role == config.API {
		go productAPI.CoverageLoop(ctx)
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	slog.Info("service listening", "service", role, "address", cfg.ListenAddr)
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
