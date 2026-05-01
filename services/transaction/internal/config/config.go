package config

import "os"

type Config struct {
	HTTPAddr       string
	GRPCAddr       string
	PostgresDSN    string
	AuthGRPCAddr   string
	SwaggerFile    string
	FrontendOrigin string
}

func Load() Config {
	return Config{
		HTTPAddr:       getEnv("HTTP_ADDR", ":8081"),
		GRPCAddr:       getEnv("GRPC_ADDR", ":9091"),
		PostgresDSN:    getEnv("POSTGRES_DSN", "postgres://fintech:fintech@localhost:5432/ledgerdb?sslmode=disable"),
		AuthGRPCAddr:   getEnv("AUTH_GRPC_ADDR", "localhost:9090"),
		SwaggerFile:    getEnv("SWAGGER_FILE", "docs/swagger.yaml"),
		FrontendOrigin: getEnv("FRONTEND_ORIGIN", "http://localhost:3000"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
