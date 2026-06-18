package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	Port        string
	HMACSecret  string
	AdminKey    string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // silently ok if .env missing in prod

	cfg := &Config{
		DatabaseURL: os.Getenv("DB_URL"),
		Port:        os.Getenv("PORT"),
		HMACSecret:  os.Getenv("HMAC_SECRET"),
		AdminKey:    os.Getenv("ADMIN_KEY"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DB_URL is required")
	}
	if cfg.HMACSecret == "" {
		return nil, fmt.Errorf("HMAC_SECRET is required")
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.AdminKey == "" {
		return nil, fmt.Errorf("ADMIN_KEY is required")
	}
	return cfg, nil
}