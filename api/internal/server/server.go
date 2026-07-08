package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/ignorant05/control/api/internal/hub"
	"github.com/ignorant05/control/api/internal/service"
	"github.com/ignorant05/control/api/internal/store"
)

type Config struct {
	Port         string
	PostgresURL  string
	RedisAddr    string
	RedisPass    string
	RedisDB      int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	JWTSecret    string
	JWTExpiry    time.Duration
}

type Server struct {
	httpServer *http.Server
	handler    *Handler
	hub        *hub.Hub
	store      store.Store
	redis      *store.RedisStore
}

func NewServer(cfg *Config) (*Server, error) {
	ctx := context.Background()

	pgStore, err := store.NewPostgresStore(ctx, cfg.PostgresURL)
	if err != nil {
		return nil, fmt.Errorf("init postgres: %w", err)
	}

	var redisStore *store.RedisStore
	if cfg.RedisAddr != "" {
		redisStore, err = store.NewRedisStore(cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB)
		if err != nil {
			return nil, fmt.Errorf("init redis: %w", err)
		}
	}

	h := hub.NewHub(redisStore)
	go h.Run(ctx)

	authService := service.NewAuthService(pgStore, cfg.JWTSecret, cfg.JWTExpiry)
	flagService := service.NewFlagService(pgStore, redisStore, h)
	userService := service.NewUserService(pgStore, redisStore, h)
	projectService := service.NewProjectService(pgStore, redisStore)

	handler := &Handler{
		AuthService: authService, FlagService: flagService,
		UserService: userService, ProjectService: projectService,
		Store: pgStore, Hub: h,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/login", handler.Login)
	mux.HandleFunc("/api/v1/register-app", handler.RegisterApp)
	mux.HandleFunc("/api/v1/projects", handler.ListProjects)
	mux.HandleFunc("/api/v1/project", handler.GetProject)
	mux.HandleFunc("/api/v1/flags", handler.ListFlags)
	mux.HandleFunc("/api/v1/flag", handler.GetFlag)
	mux.HandleFunc("/api/v1/flag/create", handler.CreateFlag)
	mux.HandleFunc("/api/v1/flag/update", handler.UpdateFlag)
	mux.HandleFunc("/api/v1/flag/toggle", handler.ToggleFlag)
	mux.HandleFunc("/api/v1/flag/rollout", handler.UpdateRollout)
	mux.HandleFunc("/api/v1/flag/kill", handler.KillFlag)
	mux.HandleFunc("/api/v1/flag/delete", handler.DeleteFlag)
	mux.HandleFunc("/api/v1/users", handler.ListUsers)
	mux.HandleFunc("/api/v1/user/create", handler.CreateUser)
	mux.HandleFunc("/api/v1/user/role", handler.UpdateUserRole)
	mux.HandleFunc("/api/v1/user/password", handler.UpdateUserPassword)
	mux.HandleFunc("/api/v1/user/delete", handler.DeleteUser)
	mux.HandleFunc("/api/v1/user/presence", handler.TogglePresence)
	mux.HandleFunc("/api/v1/audit", handler.GetAuditLog)

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, `{"error": "token required"}`, http.StatusUnauthorized)
			return
		}
		user, err := authService.ValidateToken(token)
		if err != nil {
			http.Error(w, `{"error": "invalid token"}`, http.StatusUnauthorized)
			return
		}
		project, err := projectService.GetProjectByName(r.Context(), user.Project)
		if err != nil {
			http.Error(w, `{"error": "project not found"}`, http.StatusInternalServerError)
			return
		}
		h.ServeWSByProjectID(w, r, user, project.ID)
	})

	mux.HandleFunc("/health", handler.HealthCheck)

	var handlerChain http.Handler = mux
	handlerChain = AuthMiddleware(authService)(handlerChain)
	handlerChain = CORSMiddleware(handlerChain)
	handlerChain = LoggingMiddleware(handlerChain)

	httpServer := &http.Server{
		Addr: ":" + cfg.Port, Handler: handlerChain,
		ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout, IdleTimeout: cfg.IdleTimeout,
	}

	return &Server{httpServer: httpServer, handler: handler, hub: h, store: pgStore, redis: redisStore}, nil
}

func (s *Server) Start() error {
	fmt.Printf("Server starting on %s\n", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.redis != nil {
		s.redis.Close()
	}
	if s.store != nil {
		s.store.Close()
	}
	return s.httpServer.Shutdown(ctx)
}
