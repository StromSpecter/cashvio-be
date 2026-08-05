package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
}

type ServerConfig struct {
	Port string
	Mode string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type JWTConfig struct {
	Secret    string
	ExpiresIn int
}

func Load() (*Config, error) {
	loaded := false
	for _, path := range []string{".env", "../../.env", "../../../.env"} {
		if _, err := os.Stat(path); err == nil {
			if err := godotenv.Overload(path); err != nil {
				return nil, fmt.Errorf("failed to load %s: %w", path, err)
			}
			loaded = true
			break
		}
	}
	if !loaded {
		return nil, fmt.Errorf("no .env file found in current dir or parent dirs")
	}

	expiresIn, err := strconv.Atoi(getEnv("JWT_EXPIRES_IN", "24h"))
	if err != nil {
		expiresIn = 24
	}

	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
			Mode: getEnv("GIN_MODE", "debug"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			DBName:   getEnv("DB_NAME", "cashvio"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		JWT: JWTConfig{
			Secret:    getEnv("JWT_SECRET", "changeme"),
			ExpiresIn: expiresIn,
		},
	}, nil
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode)
}

func (c *DatabaseConfig) RootDSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.SSLMode)
}
