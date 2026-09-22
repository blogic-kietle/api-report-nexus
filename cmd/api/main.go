package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api-report-nexus/internal/config"
	amqpdelivery "api-report-nexus/internal/delivery/amqp"
	httpapi "api-report-nexus/internal/delivery/http"
	"api-report-nexus/internal/domain/email"
	"api-report-nexus/internal/infrastructure/gotenberg"
	"api-report-nexus/internal/infrastructure/licenseapi"
	"api-report-nexus/internal/infrastructure/rabbitmq"
	"api-report-nexus/internal/infrastructure/session"
	"api-report-nexus/internal/usecase/scheduledmail"
)

var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)

	var mail email.Sender
	if cfg.LicenseAPIURL != "" {
		mail = licenseapi.New(cfg.LicenseAPIURL, cfg.LicenseAPITimeout)
	} else {
		log.Warn("LICENSE_API_URL not set, send-email endpoints disabled")
	}
	if cfg.Debug {
		log.Warn("DEBUG set: /debug/pprof is exposed")
	}

	deps := httpapi.Deps{
		Version: version, Debug: cfg.Debug, Log: log, Now: time.Now,
		PDF: gotenberg.New(cfg.GotenbergURL), PDFTimeout: cfg.PDFTimeout,
		Sessions:        session.New(cfg.StreamTTL),
		FinalizeTimeout: cfg.FinalizeTimeout,
		MaxChunk:        cfg.MaxChunkSize,
		ChunkSize:       cfg.PDFChunkSize,
		Mail:            mail,
		ClientURL:       cfg.ClientURL,
	}

	var bus *rabbitmq.Publisher
	if cfg.RabbitMQHost != "" {
		bus, err = rabbitmq.New(rabbitmq.Config{
			Host: cfg.RabbitMQHost, Port: cfg.RabbitMQPort, User: cfg.RabbitMQUser, Pass: cfg.RabbitMQPass, VHost: cfg.RabbitMQVHost,
			CertPath: cfg.RabbitMQCertPath, CertPass: cfg.RabbitMQCertPass, CAPath: cfg.RabbitMQCAPath, ServerName: cfg.RabbitMQServerName,
			Timeout: cfg.RabbitMQTimeout, Log: log,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "rabbitmq:", err)
			os.Exit(1)
		}
		defer func() { _ = bus.Close() }()
		deps.Notifier = bus
	} else {
		log.Warn("RABBITMQ_HOST not set, delivery report updates are not published")
	}

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           http.MaxBytesHandler(httpapi.New(deps), cfg.MaxJSONBody),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if bus != nil && mail != nil {
		go bus.Consume(ctx, rabbitmq.MailQueue, amqpdelivery.Mail(ctx, bus, scheduledmail.Service{
			Mail: mail, PDF: deps.PDF, ClientURL: cfg.ClientURL, Now: time.Now,
		}, log))
	}

	go func() {
		log.Info("report-nexus listening", "addr", srv.Addr, "env", cfg.AppEnv, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed, forcing close", "err", err)
		_ = srv.Close()
	}
}
