package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                 string
	DatabaseURL          string
	GiteaURL             string
	GiteaClientID        string
	GiteaClientSecret    string
	GiteaRedirectURL     string
	GiteaAdminToken      string
	JWTSecret            string
	FrontendURL          string
	AdminUsernames       []string
	ReviewPendingMinutes int
	WebhookBaseURL       string
	GiteaWebhookSecret   string
	GoogleCredentials    string
	GoogleSheetID        string
}

func Load() (*Config, error) {
	godotenv.Load()

	adminUsernames := []string{}
	if admins := getEnv("ADMIN_USERNAMES", ""); admins != "" {
		for _, u := range strings.Split(admins, ",") {
			adminUsernames = append(adminUsernames, strings.TrimSpace(u))
		}
	}

	reviewPendingMinutes := 15
	if rpm := getEnv("REVIEW_PENDING_MINUTES", ""); rpm != "" {
		if val := parseInt(rpm, 15); val > 0 {
			reviewPendingMinutes = val
		}
	}

	return &Config{
		Port:                 getEnv("PORT", "8080"),
		DatabaseURL:          getEnv("DATABASE_URL", ""),
		GiteaURL:             getEnv("GITEA_URL", ""),
		GiteaClientID:        getEnv("GITEA_CLIENT_ID", ""),
		GiteaClientSecret:    getEnv("GITEA_CLIENT_SECRET", ""),
		GiteaRedirectURL:     getEnv("GITEA_REDIRECT_URL", ""),
		GiteaAdminToken:      getEnv("GITEA_ADMIN_TOKEN", ""),
		JWTSecret:            getEnv("JWT_SECRET", ""),
		FrontendURL:          getEnv("FRONTEND_URL", "http://localhost:5173"),
		AdminUsernames:       adminUsernames,
		ReviewPendingMinutes: reviewPendingMinutes,
		WebhookBaseURL:       getEnv("WEBHOOK_BASE_URL", ""),
		GiteaWebhookSecret:   getEnv("GITEA_WEBHOOK_SECRET", ""),
		GoogleCredentials:    getEnv("GOOGLE_CREDENTIALS_FILE", ""),
		GoogleSheetID:        getEnv("GOOGLE_SHEET_ID", ""),
	}, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseInt(s string, fallback int) int {
	if val, err := strconv.Atoi(s); err == nil {
		return val
	}
	return fallback
}
