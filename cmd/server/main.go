package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ai-shortlink/internal/config"
	"ai-shortlink/internal/dbutil"
	"ai-shortlink/internal/server"
	"ai-shortlink/internal/store"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	db, err := dbutil.Open(ctx, cfg.DatabaseMode, cfg.DSN, cfg.SQLitePath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if cfg.AutoMigrate {
		if err := dbutil.Migrate(ctx, db, cfg.DatabaseMode); err != nil {
			log.Fatalf("migrate: %v", err)
		}
	}

	srv, err := server.New(cfg, store.New(db, cfg.DatabaseMode))
	if err != nil {
		log.Fatalf("new server: %v", err)
	}

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 8 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	go func() {
		log.Printf("%s listening on %s", cfg.AppName, cfg.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Println("shutting down")
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
