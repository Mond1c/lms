package main

import (
	"io/fs"
	"log"
	"net/http"

	"github.com/Mond1c/gitea-classroom/config"
	"github.com/Mond1c/gitea-classroom/frontend"
	"github.com/Mond1c/gitea-classroom/internal/cache"
	"github.com/Mond1c/gitea-classroom/internal/database"
	"github.com/Mond1c/gitea-classroom/internal/handlers"
	mw "github.com/Mond1c/gitea-classroom/internal/middleware"
	"github.com/Mond1c/gitea-classroom/internal/services"
	"github.com/Mond1c/gitea-classroom/internal/workers"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	if err := database.Connect(cfg.DatabaseURL); err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	if err := database.Migrate(); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	// Initialize Google Sheets service (optional)
	var sheetsService *services.SheetsService
	if cfg.GoogleCredentials != "" && cfg.GoogleSheetID != "" {
		var err error
		sheetsService, err = services.NewSheetsService(cfg.GoogleCredentials, cfg.GoogleSheetID)
		if err != nil {
			log.Printf("Warning: Failed to initialize Google Sheets service: %v", err)
		} else {
			log.Println("Google Sheets service initialized")
		}
	}

	// Initialize review cache and worker
	reviewCache := cache.NewReviewCache()
	reviewWorker := workers.NewReviewWorker(reviewCache, sheetsService)
	reviewWorker.Start()
	defer reviewWorker.Stop()

	e := echo.New()

	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{cfg.FrontendURL},
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	authHandler := handlers.NewAuthHandler(cfg)
	courseHandler := handlers.NewCourseHandler(cfg)
	assignmentHandler := handlers.NewAssignmentHandler(cfg)
	studentHandler := handlers.NewStudentHandler(cfg)
	submissionHandler := handlers.NewSubmissionHandler(cfg)
	reviewHandler := handlers.NewReviewHandler(cfg, reviewCache, sheetsService)
	webhookHandler := handlers.NewWebhookHandler(cfg, reviewCache, sheetsService)
	inviteHandler := handlers.NewInviteHandler(cfg)

	e.GET("/api/health", func(c echo.Context) error {
		return c.JSON(200, map[string]string{"status": "ok"})
	})
	e.GET("/api/auth/login", authHandler.Login)
	e.GET("/api/auth/callback", authHandler.Callback)
	e.GET("/api/invite/:code", courseHandler.GetByInviteCode)
	e.POST("/api/webhooks/gitea", webhookHandler.HandleGiteaWebhook)

	// Public invite endpoints (no auth required)
	e.GET("/api/join/:code", inviteHandler.GetAvailableStudents)
	e.POST("/api/join/:code/register", inviteHandler.RegisterStudent)

	api := e.Group("/api")
	api.Use(mw.AuthMiddleware(cfg.JWTSecret))

	api.GET("/auth/me", authHandler.Me)

	api.POST("/courses", courseHandler.Create)
	api.GET("/courses", courseHandler.List)
	api.GET("/courses/enrolled", courseHandler.ListEnrolled)
	api.GET("/courses/:slug", courseHandler.Get)
	api.POST("/courses/:slug/regenerate-invite", courseHandler.RegenerateInviteCode)

	api.GET("/courses/:slug/assignments", assignmentHandler.List)
	api.POST("/courses/:slug/assignments", assignmentHandler.Create)
	api.GET("/assignments/:id", assignmentHandler.Get)
	api.PUT("/assignments/:id", assignmentHandler.Update)
	api.DELETE("/assignments/:id", assignmentHandler.Delete)

	api.GET("/courses/:slug/students", studentHandler.List)
	api.POST("/courses/:slug/enroll", studentHandler.Enroll)
	api.GET("/students/:id", studentHandler.Get)
	api.DELETE("/students/:id", studentHandler.Remove)

	// Student invite management
	api.POST("/courses/:slug/students/import", inviteHandler.ImportStudents)
	api.GET("/courses/:slug/invites", inviteHandler.ListInvites)

	api.POST("/assignments/:id/accept", submissionHandler.Accept)
	api.GET("/assignments/:id/submissions", submissionHandler.List)
	api.GET("/submissions/:submissionId", submissionHandler.Get)
	api.POST("/submissions/:submissionId/grade", submissionHandler.Grade)

	// Review endpoints
	api.POST("/submissions/:id/review/request", reviewHandler.RequestReview)
	api.DELETE("/reviews/:id/cancel", reviewHandler.CancelReview)
	api.GET("/submissions/:id/review/status", reviewHandler.GetReviewStatus)
	api.POST("/reviews/:id/mark-reviewed", reviewHandler.MarkReviewed)

	// Serve embedded frontend (SPA)
	distFS, err := fs.Sub(frontend.DistFS, "dist")
	if err != nil {
		log.Fatal("Failed to get frontend dist fs:", err)
	}
	fileServer := http.FileServer(http.FS(distFS))
	e.GET("/*", echo.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try serving the file directly
		f, err := distFS.(fs.ReadFileFS).ReadFile(r.URL.Path[1:])
		if err != nil {
			// File not found — serve index.html for SPA routing
			index, _ := distFS.(fs.ReadFileFS).ReadFile("index.html")
			w.Header().Set("Content-Type", "text/html")
			w.Write(index)
			return
		}
		_ = f
		fileServer.ServeHTTP(w, r)
	})))

	log.Printf("Server starting on port %s", cfg.Port)
	e.Logger.Fatal(e.Start(":" + cfg.Port))
}
