package main

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/example/fintech-core-api/services/transaction/internal/config"
	grpcdelivery "github.com/example/fintech-core-api/services/transaction/internal/delivery/grpc"
	httpapi "github.com/example/fintech-core-api/services/transaction/internal/delivery/http"
	"github.com/example/fintech-core-api/services/transaction/internal/events"
	postgresrepo "github.com/example/fintech-core-api/services/transaction/internal/repository/postgres"
	"github.com/example/fintech-core-api/services/transaction/internal/usecase"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	cfg := config.Load()

	db, err := sql.Open("pgx", cfg.PostgresDSN)
	if err != nil {
		logger.Fatal("open postgres", zap.Error(err))
	}
	defer db.Close()
	db.SetMaxOpenConns(30)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Fatal("ping postgres", zap.Error(err))
	}

	authClient, err := grpcdelivery.NewAuthClient(cfg.AuthGRPCAddr, 5*time.Second)
	if err != nil {
		logger.Fatal("create auth grpc client", zap.Error(err))
	}
	defer authClient.Close()

	broker := events.NewBroker()
	ledgerRepo := postgresrepo.NewLedgerRepository(db)
	ledgerUsecase := usecase.NewTransactionUsecase(ledgerRepo, broker)

	httpServer := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      httpapi.NewHandler(ledgerUsecase, authClient, broker, logger, cfg.SwaggerFile, cfg.FrontendOrigin),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  60 * time.Second,
	}

	grpcServer := grpc.NewServer()
	grpcdelivery.RegisterLedgerService(grpcServer, grpcdelivery.NewLedgerServer(ledgerUsecase))
	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		logger.Fatal("listen grpc", zap.String("addr", cfg.GRPCAddr), zap.Error(err))
	}

	errCh := make(chan error, 2)
	go func() {
		logger.Info("transaction http server started", zap.String("addr", cfg.HTTPAddr))
		errCh <- httpServer.ListenAndServe()
	}()
	go func() {
		logger.Info("transaction grpc server started", zap.String("addr", cfg.GRPCAddr))
		errCh <- grpcServer.Serve(grpcListener)
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-signalCh:
		logger.Info("shutdown signal received", zap.String("signal", sig.String()))
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, grpc.ErrServerStopped) {
			logger.Fatal("server failed", zap.Error(err))
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown http", zap.Error(err))
	}
	grpcServer.GracefulStop()
}
