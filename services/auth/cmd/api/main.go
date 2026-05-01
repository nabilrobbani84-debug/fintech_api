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

	"github.com/example/fintech-core-api/services/auth/internal/config"
	grpcdelivery "github.com/example/fintech-core-api/services/auth/internal/delivery/grpc"
	httpapi "github.com/example/fintech-core-api/services/auth/internal/delivery/http"
	mysqlrepo "github.com/example/fintech-core-api/services/auth/internal/repository/mysql"
	"github.com/example/fintech-core-api/services/auth/internal/security"
	"github.com/example/fintech-core-api/services/auth/internal/usecase"
	_ "github.com/go-sql-driver/mysql"
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

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("load config", zap.Error(err))
	}

	db, err := sql.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		logger.Fatal("open mysql", zap.Error(err))
	}
	defer db.Close()
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Fatal("ping mysql", zap.Error(err))
	}

	encryptor, err := security.NewEncryptor(cfg.EncryptionKey)
	if err != nil {
		logger.Fatal("create encryptor", zap.Error(err))
	}

	tokens := security.NewJWTManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTTTL)
	users := mysqlrepo.NewUserRepository(db)
	authUsecase := usecase.NewAuthUsecase(users, encryptor, tokens)

	httpServer := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      httpapi.NewHandler(authUsecase, logger, cfg.SwaggerFile, cfg.FrontendOrigin),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	grpcServer := grpc.NewServer()
	grpcdelivery.RegisterAuthService(grpcServer, grpcdelivery.NewAuthServer(authUsecase))
	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		logger.Fatal("listen grpc", zap.String("addr", cfg.GRPCAddr), zap.Error(err))
	}

	errCh := make(chan error, 2)
	go func() {
		logger.Info("auth http server started", zap.String("addr", cfg.HTTPAddr))
		errCh <- httpServer.ListenAndServe()
	}()
	go func() {
		logger.Info("auth grpc server started", zap.String("addr", cfg.GRPCAddr))
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
