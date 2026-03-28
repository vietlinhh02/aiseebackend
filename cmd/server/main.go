package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/team-dang/aiseebackend/internal/auth"
	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/internal/user"
	"github.com/team-dang/aiseebackend/pkg/response"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://aisee:aisee@localhost:5432/aisee?sslmode=disable"
	}

	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	internalKey := os.Getenv("INTERNAL_API_KEY")

	authService := auth.NewService(pool, jwtSecret)
	authHandler := auth.NewHandler(authService)

	userService := user.NewService(pool)
	userHandler := user.NewHandler(userService)

	r := chi.NewRouter()
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			response.Err(w, http.StatusServiceUnavailable, "db_error", "database unreachable")
			return
		}
		response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/google", authHandler.GoogleAuth)
		r.Post("/auth/refresh", authHandler.Refresh)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(jwtSecret))
			r.Get("/users/me", userHandler.GetMe)
			r.Post("/users/family-link", userHandler.CreateFamilyLink)
			r.Post("/users/join-family", userHandler.JoinFamily)
			r.Get("/users/me/family", userHandler.GetFamily)
		})
	})

	r.Route("/api/internal", func(r chi.Router) {
		r.Use(middleware.InternalAuth(internalKey))
		// Internal endpoints added in later tasks
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	slog.Info("starting server", "port", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
