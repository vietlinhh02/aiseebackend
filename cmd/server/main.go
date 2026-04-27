package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/team-dang/aiseebackend/internal/appointment"
	"github.com/team-dang/aiseebackend/internal/assistant"
	"github.com/team-dang/aiseebackend/internal/auth"
	"github.com/team-dang/aiseebackend/internal/billing"
	"github.com/team-dang/aiseebackend/internal/call"
	"github.com/team-dang/aiseebackend/internal/chat"
	"github.com/team-dang/aiseebackend/internal/dashboard"
	"github.com/team-dang/aiseebackend/internal/dbgen"
	"github.com/team-dang/aiseebackend/internal/emergency"
	"github.com/team-dang/aiseebackend/internal/health"
	"github.com/team-dang/aiseebackend/internal/medication"
	"github.com/team-dang/aiseebackend/internal/middleware"
	"github.com/team-dang/aiseebackend/internal/music"
	"github.com/team-dang/aiseebackend/internal/toolcall"
	"github.com/team-dang/aiseebackend/internal/upload"
	"github.com/team-dang/aiseebackend/internal/user"
	"github.com/team-dang/aiseebackend/internal/volunteer"
	lk "github.com/team-dang/aiseebackend/pkg/livekit"
	"github.com/team-dang/aiseebackend/pkg/push"
	"github.com/team-dang/aiseebackend/pkg/reminder"
	"github.com/team-dang/aiseebackend/pkg/response"
	"github.com/team-dang/aiseebackend/pkg/storage"
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

	// Redis
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379/0"
	}
	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		slog.Error("invalid REDIS_URL", "error", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(redisOpts)
	defer rdb.Close()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		slog.Warn("redis not available, reminders disabled",
			"error", err,
		)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	internalKey := os.Getenv("INTERNAL_API_KEY")
	googleWebClientID := os.Getenv("GOOGLE_WEB_CLIENT_ID")

	if googleWebClientID != "" {
		slog.Info("Google Sign-In configured", "web_client_id", googleWebClientID)
	} else {
		slog.Warn("GOOGLE_WEB_CLIENT_ID not set, skipping audience verification")
	}

	// Push notification senders
	pusher := initPusher()

	// S3 / MinIO storage
	storageClient, err := storage.NewClient(storage.Config{
		Endpoint:       os.Getenv("S3_ENDPOINT"),
		PublicEndpoint: os.Getenv("S3_PUBLIC_ENDPOINT"),
		AccessKey:      os.Getenv("S3_ACCESS_KEY"),
		SecretKey:      os.Getenv("S3_SECRET_KEY"),
		Bucket:         os.Getenv("S3_BUCKET"),
		UseSSL:         os.Getenv("S3_USE_SSL") == "true",
	})
	if err != nil {
		slog.Error("storage client init failed", "error", err)
		os.Exit(1)
	}
	if err := storageClient.EnsureBucket(
		context.Background(),
	); err != nil {
		slog.Warn(
			"bucket setup failed, uploads may not work",
			"error", err,
		)
	}

	uploadHandler := upload.NewHandler(storageClient)

	// Services
	queries := dbgen.New(pool)
	authService := auth.NewService(pool, rdb, jwtSecret)
	authHandler := auth.NewHandler(authService)
	assistantBridge := assistant.NewWebSocketBridge(os.Getenv("ASSISTANT_BRIDGE_URL"))
	assistantService := assistant.NewServiceWithBridge(jwtSecret, assistantBridge)
	assistantHandler := assistant.NewHandler(assistantService)

	userService := user.NewService(pool)
	userHandler := user.NewHandler(userService)
	billingService := billing.NewService(billing.NewPgxStore(pool))
	billingHandler := billing.NewHandlerWithConfig(
		billingService,
		os.Getenv("REVENUECAT_WEBHOOK_AUTH"),
		billing.RevenueCatConfig{
			ProductPlanMap: os.Getenv("REVENUECAT_PRODUCT_PLAN_MAP"),
		},
	)

	lkClient := lk.NewClient(
		os.Getenv("LIVEKIT_URL"),
		os.Getenv("LIVEKIT_API_KEY"),
		os.Getenv("LIVEKIT_API_SECRET"),
	)

	chatHub := chat.NewHub()
	callService := call.NewService(lkClient, pusher, chatHub, billingService)
	callHandler := call.NewHandler(callService, userService)

	chatService := chat.NewService(pool, chatHub, pusher)
	chatHandler := chat.NewHandler(
		chatService, chatHub, userService, jwtSecret,
	)

	// Reminder scheduler
	scheduler := reminder.New(rdb, 30*time.Second)

	// Health services
	healthService := health.NewService(queries)
	medService := medication.NewService(
		queries, scheduler, pusher,
	)
	aptService := appointment.NewService(
		queries, scheduler, pusher,
	)

	// Register reminder handlers
	scheduler.Register("medications", medService)
	scheduler.Register("appointments", aptService)

	// Start scheduler in background
	go func() {
		if err := scheduler.Start(context.Background()); err != nil {
			slog.Error("scheduler failed", "error", err)
		}
	}()
	defer scheduler.Stop()

	emergService := emergency.NewService(queries, pusher, lkClient)
	musicService := music.NewService(publicBaseURL())

	// Dashboard
	dashService := dashboard.NewService(queries)
	dashHandler := dashboard.NewHandler(dashService, userService)
	meHandler := dashboard.NewMeHandler(dashService)

	// Volunteer
	volService := volunteer.NewService(
		queries, rdb, lkClient, pusher, billingService,
	)
	volHandler := volunteer.NewHandler(volService)

	toolHandler := toolcall.NewHandler(
		chatService, callService, userService,
		medService, healthService, aptService,
		emergService, musicService,
		volService,
		billingService,
		queries,
	)

	// Router
	r := chi.NewRouter()
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(middleware.RequestLog)
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
		r.Get("/plans", billingHandler.ListPlans)
		r.Get("/music/stream/{id}", musicService.Stream)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(jwtSecret, rdb))

			// Users
			r.Get("/users/me", userHandler.GetMe)
			r.Put("/users/me", userHandler.UpdateMe)
			r.Post("/users/family-link", userHandler.CreateFamilyLink)
			r.Delete("/users/family-link/{link_id}", userHandler.DeleteFamilyLink)
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

			// Upload
			r.Post("/upload", uploadHandler.Upload)

			// AI assistant
			r.Post("/ai/session", assistantHandler.CreateSession)

			// My health (user's own data)
			r.Route("/me", func(r chi.Router) {
				r.Get("/billing-overview", billingHandler.CurrentBillingOverview)
				r.Get("/subscription", billingHandler.CurrentSubscription)
				r.Get("/entitlements", billingHandler.CurrentEntitlements)
				r.Get("/usage", billingHandler.CurrentUsage)
				r.Get("/medications", meHandler.Medications)
				r.Get("/appointments", meHandler.Appointments)
				r.Get("/health", meHandler.Health)
			})

			// Dashboard (family)
			r.Route("/dashboard/{user_id}", func(r chi.Router) {
				r.Get("/health", dashHandler.Health)
				r.Get("/medications", dashHandler.Medications)
				r.Get("/audit-log", dashHandler.AuditLog)
				r.Get("/location", dashHandler.Location)
				r.Post("/medications", dashHandler.CreateMedication)
				r.Put("/medications/{med_id}", dashHandler.UpdateMedication)
				r.Delete("/medications/{med_id}", dashHandler.DeleteMedication)
			})

			// Volunteers
			r.Route("/volunteers", func(r chi.Router) {
				r.Get("/status", volHandler.GetStatus)
				r.Post("/register", volHandler.Register)
				r.Put("/availability", volHandler.SetAvailability)
				r.Post("/match", volHandler.StartMatch)
				r.Get("/match/{match_id}/status", volHandler.MatchStatus)
				r.Post("/rate", volHandler.Rate)
			})
		})

		// WebSocket (auth via query param, not middleware)
		r.Get("/ai/ws", assistantHandler.WebSocket)
		r.Get("/ws/chat", chatHandler.WebSocket)
	})

	r.Route("/api/internal", func(r chi.Router) {
		r.Use(middleware.InternalAuth(internalKey))
		r.Post("/tool-call", toolHandler.HandleToolCall)
		r.Post("/usage/check", billingHandler.CheckQuota)
		r.Post("/usage/record", billingHandler.RecordUsage)
		r.Post("/usage/session/start", billingHandler.StartUsageSession)
		r.Post("/usage/session/commit", billingHandler.CommitUsageSession)
		r.Post("/usage/grant", billingHandler.GrantUsageQuota)
	})

	r.Post("/api/webhooks/revenuecat", billingHandler.RevenueCatWebhook)

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

func publicBaseURL() string {
	if value := os.Getenv("PUBLIC_API_BASE_URL"); value != "" {
		return value
	}
	if value := os.Getenv("APP_PUBLIC_BASE_URL"); value != "" {
		return value
	}
	if domain := os.Getenv("DOMAIN"); domain != "" {
		return "https://" + domain
	}
	return ""
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
