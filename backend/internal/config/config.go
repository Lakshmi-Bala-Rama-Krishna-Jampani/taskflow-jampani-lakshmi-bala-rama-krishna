package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port         string
	DatabaseURL  string
	JWTSecret    string
	BcryptCost   int
	MigrationsPath string
}

func Load() (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	cost := 12
	if v := os.Getenv("BCRYPT_COST"); v != "" {
		c, err := strconv.Atoi(v)
		if err != nil || c < 12 {
			return nil, fmt.Errorf("BCRYPT_COST must be an integer >= 12")
		}
		cost = c
	}
	migrations := os.Getenv("MIGRATIONS_PATH")
	if migrations == "" {
		migrations = "./migrations"
	}
	return &Config{
		Port:           port,
		DatabaseURL:  dbURL,
		JWTSecret:    secret,
		BcryptCost:   cost,
		MigrationsPath: migrations,
	}, nil
}
