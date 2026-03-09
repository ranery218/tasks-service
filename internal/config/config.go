package config

import (
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	defaultHTTPAddr        = ":8080"
	defaultShutdownTimeout = 10 * time.Second
)

type Config struct {
	HTTPAddr        string
	ShutdownTimeout time.Duration
	MySQLDSN        string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	JWTAccessSecret string
	JWTAccessTTL    time.Duration
	JWTRefreshTTL   time.Duration
	BcryptCost      int
}

func FromEnv() Config {
	cfg := Config{
		HTTPAddr:        getEnv("HTTP_ADDR", defaultHTTPAddr),
		ShutdownTimeout: defaultShutdownTimeout,
		MySQLDSN:        getEnv("MYSQL_DSN", ""),
		RedisAddr:       getEnv("REDIS_ADDR", ""),
		RedisPassword:   getEnv("REDIS_PASSWORD", ""),
		RedisDB:         0,
		JWTAccessSecret: getEnv("JWT_ACCESS_SECRET", "dev_access_secret"),
		JWTAccessTTL:    60 * time.Minute,
		JWTRefreshTTL:   30 * 24 * time.Hour,
		BcryptCost:      bcrypt.DefaultCost,
	}

	if value := os.Getenv("SHUTDOWN_TIMEOUT_SEC"); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
			cfg.ShutdownTimeout = time.Duration(seconds) * time.Second
		}
	}
	if value := os.Getenv("JWT_ACCESS_TTL_SEC"); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
			cfg.JWTAccessTTL = time.Duration(seconds) * time.Second
		}
	}
	if value := os.Getenv("JWT_REFRESH_TTL_SEC"); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
			cfg.JWTRefreshTTL = time.Duration(seconds) * time.Second
		}
	}
	if value := os.Getenv("BCRYPT_COST"); value != "" {
		if cost, err := strconv.Atoi(value); err == nil && cost >= bcrypt.MinCost && cost <= bcrypt.MaxCost {
			cfg.BcryptCost = cost
		}
	}
	if value := os.Getenv("REDIS_DB"); value != "" {
		if db, err := strconv.Atoi(value); err == nil && db >= 0 {
			cfg.RedisDB = db
		}
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
