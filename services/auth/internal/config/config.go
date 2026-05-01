package config

import (
	"encoding/base64"
	"errors"
	"os"
	"time"
)

type Config struct {
	HTTPAddr       string
	GRPCAddr       string
	MySQLDSN       string
	JWTSecret      string
	JWTIssuer      string
	JWTTTL         time.Duration
	EncryptionKey  []byte
	SwaggerFile    string
	FrontendOrigin string
}

func Load() (Config, error) {
	key, err := encryptionKey(getEnv("ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		HTTPAddr:       getEnv("HTTP_ADDR", ":8080"),
		GRPCAddr:       getEnv("GRPC_ADDR", ":9090"),
		MySQLDSN:       getEnv("MYSQL_DSN", "fintech:fintech@tcp(localhost:3306)/authdb?parseTime=true&multiStatements=false"),
		JWTSecret:      getEnv("JWT_SECRET", "change-me-with-a-strong-secret"),
		JWTIssuer:      getEnv("JWT_ISSUER", "fintech-core-api"),
		JWTTTL:         getDurationEnv("JWT_TTL", 2*time.Hour),
		EncryptionKey:  key,
		SwaggerFile:    getEnv("SWAGGER_FILE", "docs/swagger.yaml"),
		FrontendOrigin: getEnv("FRONTEND_ORIGIN", "http://localhost:3000"),
	}, nil
}

func encryptionKey(raw string) ([]byte, error) {
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if len(raw) == 32 {
		return []byte(raw), nil
	}
	return nil, errors.New("ENCRYPTION_KEY must be 32 raw bytes or base64-encoded 32 bytes")
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
