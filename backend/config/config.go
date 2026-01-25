package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port              string
	DatabaseURL       string
	GiteaURL          string
	GiteaClientID     string
	GiteaClientSecret string
	GiteaRedirectURL  string
	JWTSecret         string
	FrontendURL       string
	AdminUsernames    []string
}

func Load() (*Config, error) {
	godotenv.Load()

	adminUsernames := []string{}
	if admins := getEnv("ADMIN_USERNAMES", ""); admins != "" {
		for _, u := range strings.Split(admins, ",") {
			adminUsernames = append(adminUsernames, strings.TrimSpace(u))
		}
	}

	return &Config{
		Port:              getEnv("PORT", "8080"),
		DatabaseURL:       getEnv("DATABASE_URL", ""),
		GiteaURL:          getEnv("GITEA_URL", ""),
		GiteaClientID:     getEnv("GITEA_CLIENT_ID", ""),
		GiteaClientSecret: getEnv("GITEA_CLIENT_SECRET", ""),
		GiteaRedirectURL:  getEnv("GITEA_REDIRECT_URL", ""),
		JWTSecret:         getEnv("JWT_SECRET", ""),
		FrontendURL:       getEnv("FRONTEND_URL", "http://localhost:5173"),
		AdminUsernames:    adminUsernames,
	}, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
