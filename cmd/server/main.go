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
	"github.com/team-dang/aiseebackend/internal/call"
	"github.com/team-dang/aiseebackend/internal/chat"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/internal/toolcall"
	"github.com/team-dang/aiseebackend/internal/user"
	lk "github.com/team-dang/aiseebackend/pkg/livekit"
	"github.com/team-dang/aiseebackend/pkg/push"
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

	// Push notification senders
	pusher := initPusher()

	// Services
	queries := dbgen.New(pool)
	authService := auth.NewService(pool, jwtSecret)
	authHandler := auth.NewHandler(authService)

	userService := user.NewService(pool)
	userHandler := user.NewHandler(userService)

	lkClient := lk.NewClient(
		os.Getenv("LIVEKIT_URL"),
		os.Getenv("LIVEKIT_API_KEY"),
		os.Getenv("LIVEKIT_API_SECRET"),
	)

	callService := call.NewService(lkClient, pusher)
	callHandler := call.NewHandler(callService, userService)

	chatHub := chat.NewHub()
	chatService := chat.NewService(pool, chatHub, pusher)
	chatHandler := chat.NewHandler(
		chatService, chatHub, userService, jwtSecret,
	)

	toolHandler := toolcall.NewHandler(
		chatService, callService, userService, queries,
	)

	// Router
	r := chi.NewRouter()
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			response.Err(
				w, http.StatusServiceUnavailable,
				"db_error", "database unreachable",
			)
			return
		}
		response.JSON(
			w, http.StatusOK,
			map[string]string{"status": "ok"},
		)
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/google", authHandler.GoogleAuth)
		r.Post("/auth/refresh", authHandler.Refresh)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(jwtSecret))

			// Users
			r.Get("/users/me", userHandler.GetMe)
			r.Put("/users/me", userHandler.UpdateMe)
			r.Post("/users/family-link", userHandler.CreateFamilyLink)
			r.Post("/users/join-family", userHandler.JoinFamily)
			r.Get("/users/me/family", userHandler.GetFamily)

			// Calls
			r.Post("/calls/token", callHandler.GetToken)
			r.Post("/calls/create-room", callHandler.CreateRoom)
			r.Post("/calls/end-room", callHandler.EndRoom)

			// Messages (static routes before parameterized)
			r.Get("/messages/unread-count", chatHandler.UnreadCount)
			r.Post("/messages/send", chatHandler.SendMessage)
			r.Get("/messages/{contactID}", chatHandler.GetMessages)
			r.Put("/messages/{id}/read", chatHandler.MarkRead)
		})

		// WebSocket (auth via query param, not middleware)
		r.Get("/ws/chat", chatHandler.WebSocket)
	})

	r.Route("/api/internal", func(r chi.Router) {
		r.Use(middleware.InternalAuth(internalKey))
		r.Post("/tool-call", toolHandler.HandleToolCall)
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

func initPusher() *push.Pusher {
	var fcmSender push.Sender
	var apnsSender push.Sender

	fcmPath := os.Getenv("FCM_SERVICE_ACCOUNT_PATH")
	if fcmPath != "" {
		sender, err := push.NewFCMSender(fcmPath)
		if err != nil {
			slog.Warn(
				"FCM sender init failed, push disabled",
				"error", err,
			)
		} else {
			fcmSender = sender
		}
	}

	apnsKeyPath := os.Getenv("APNS_KEY_PATH")
	if apnsKeyPath != "" {
		sender, err := push.NewAPNsSender(
			apnsKeyPath,
			os.Getenv("APNS_KEY_ID"),
			os.Getenv("APNS_TEAM_ID"),
			os.Getenv("APNS_BUNDLE_ID"),
			false,
		)
		if err != nil {
			slog.Warn(
				"APNs sender init failed, push disabled",
				"error", err,
			)
		} else {
			apnsSender = sender
		}
	}

	if fcmSender == nil {
		fcmSender = &push.NoopSender{}
	}
	if apnsSender == nil {
		apnsSender = &push.NoopSender{}
	}

	return push.NewPusher(fcmSender, apnsSender)
}
