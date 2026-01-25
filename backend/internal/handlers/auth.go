package handlers

import (
	"net/http"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/Mond1c/gitea-classroom/config"
	"github.com/Mond1c/gitea-classroom/internal/database"
	"github.com/Mond1c/gitea-classroom/internal/middleware"
	"github.com/Mond1c/gitea-classroom/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
)

type AuthHandler struct {
	cfg         *config.Config
	oauthConfig *oauth2.Config
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	oauthConfig := &oauth2.Config{
		ClientID:     cfg.GiteaClientID,
		ClientSecret: cfg.GiteaClientSecret,
		RedirectURL:  cfg.GiteaRedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  cfg.GiteaURL + "/login/oauth/authorize",
			TokenURL: cfg.GiteaURL + "/login/oauth/access_token",
		},
		Scopes: []string{"read:user", "write:repository", "write:organization"},
	}

	return &AuthHandler{
		cfg:         cfg,
		oauthConfig: oauthConfig,
	}
}

func (h *AuthHandler) Login(c echo.Context) error {
	url := h.oauthConfig.AuthCodeURL("state")
	return c.JSON(http.StatusOK, map[string]string{"url": url})
}

func (h *AuthHandler) Callback(c echo.Context) error {
	code := c.QueryParam("code")

	token, err := h.oauthConfig.Exchange(c.Request().Context(), code)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "failed to exchange token")
	}

	client, err := gitea.NewClient(h.cfg.GiteaURL, gitea.SetToken(token.AccessToken))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create gitea client")
	}

	giteaUser, _, err := client.GetMyUserInfo()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user info")
	}

	isAdmin := false
	for _, adminUsername := range h.cfg.AdminUsernames {
		if adminUsername == giteaUser.UserName {
			isAdmin = true
			break
		}
	}

	var user models.User
	result := database.DB.Where("gitea_id = ?", giteaUser.ID).First(&user)

	if result.Error != nil {
		user = models.User{
			GiteaID:      giteaUser.ID,
			Username:     giteaUser.UserName,
			Email:        giteaUser.Email,
			FullName:     giteaUser.FullName,
			AvatarURL:    giteaUser.AvatarURL,
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
			IsAdmin:      isAdmin,
		}
		database.DB.Create(&user)
	} else {
		user.AccessToken = token.AccessToken
		user.RefreshToken = token.RefreshToken
		user.IsAdmin = isAdmin
		database.DB.Save(&user)
	}

	claims := &middleware.JWTClaims{
		UserID: user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := jwtToken.SignedString([]byte(h.cfg.JWTSecret))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate token")
	}

	redirectURL := h.cfg.FrontendURL + "/auth/callback?token=" + tokenString
	return c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}

func (h *AuthHandler) Me(c echo.Context) error {
	userID := c.Get("user_id").(uint)

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	return c.JSON(http.StatusOK, user)
}
