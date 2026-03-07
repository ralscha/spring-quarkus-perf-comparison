package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/quarkusio/spring-quarkus-perf-comparison/go/internal/app"
	"github.com/quarkusio/spring-quarkus-perf-comparison/go/internal/config"
	"github.com/quarkusio/spring-quarkus-perf-comparison/go/internal/fruit"
)

func main() {
	cpuprofile := flag.String("cpuprofile", "", "write a CPU profile to the specified file")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	repository, err := fruit.NewPostgresRepository(ctx, cfg.Database)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer repository.Close()

	handler := app.NewHandler(logger, repository)
	server := &http.Server{
		Addr:         cfg.Server.Address(),
		Handler:      handler,
		ReadTimeout:  cfg.Server.ReadTimeout(),
		WriteTimeout: cfg.Server.WriteTimeout(),
		IdleTimeout:  cfg.Server.IdleTimeout(),
	}

	var profileFile *os.File
	if *cpuprofile != "" {
		profileFile, err = os.Create(*cpuprofile)
		if err != nil {
			logger.Error("failed to create CPU profile", "path", *cpuprofile, "error", err)
			os.Exit(1)
		}

		if err := pprof.StartCPUProfile(profileFile); err != nil {
			_ = profileFile.Close()
			logger.Error("failed to start CPU profile", "path", *cpuprofile, "error", err)
			os.Exit(1)
		}

		defer func() {
			pprof.StopCPUProfile()
			if err := profileFile.Close(); err != nil {
				logger.Error("failed to close CPU profile", "path", *cpuprofile, "error", err)
			}
		}()

		logger.Info("CPU profiling enabled", "path", *cpuprofile)
	}

	go func() {
		logger.Info("server started", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}
