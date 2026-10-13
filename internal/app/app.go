package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"quicklink/internal/cache"
	"quicklink/internal/config"
	"quicklink/internal/database"
	"quicklink/internal/handler"
	"quicklink/internal/store"
)

type Option func(*App)

func WithConfig(cfg *config.Config) Option {
	return func(a *App) {
		a.cfg = cfg
	}
}

func WithStore(s store.Store) Option {
	return func(a *App) {
		a.store = s
	}
}

func WithHandler(h *handler.Handler) Option {
	return func(a *App) {
		a.handler = h
	}
}

func WithDatabase(db *database.DB) Option {
	return func(a *App) {
		a.db = db
	}
}

func WithCache(c *cache.Client) Option {
	return func(a *App) {
		a.cache = c
	}
}

type App struct {
	cfg     *config.Config
	db      *database.DB
	cache   *cache.Client
	store   store.Store
	handler *handler.Handler
	srv     *http.Server
}

func New(opts ...Option) (*App, error) {
	a := &App{}

	for _, opt := range opts {
		opt(a)
	}

	if a.cfg == nil {
		a.cfg = config.New()
	}

	if a.db == nil && a.store == nil {
		db, err := database.NewPostgres(a.cfg.DatabaseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to init database: %w", err)
		}
		a.db = db
	}

	if a.cache == nil && a.cfg.RedisURL != "" {
		redisClient, err := cache.NewRedis(a.cfg.RedisURL)
		if err != nil {
			log.Printf("Warning: Redis unavailable (%v). Continuing without cache.", err)
		} else {
			a.cache = redisClient
		}
	}

	if a.store == nil && a.db != nil {
		storeOpts := []store.GormStoreOption{
			store.WithAutoMigrate(true),
		}
		if a.cache != nil {
			storeOpts = append(storeOpts, store.WithCache(a.cache))
		}

		s, err := store.NewGormStore(a.db, storeOpts...)
		if err != nil {
			return nil, fmt.Errorf("failed to init store: %w", err)
		}
		a.store = s
	}

	if a.handler == nil {
		a.handler = handler.New(
			handler.WithBaseURL(a.cfg.BaseURL),
			handler.WithStore(a.store),
		)
	}

	addr := fmt.Sprintf("%s:%s", a.cfg.Host, a.cfg.Port)
	a.srv = &http.Server{
		Addr:         addr,
		Handler:      a.handler.Engine(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return a, nil
}

func (a *App) Run() error {
	defer a.cleanup()

	errChan := make(chan error, 1)

	go func() {
		fmt.Println("==================================================")
		fmt.Println("  ⚡ QuickLink URL Shortener (Options Pattern)")
		fmt.Println("==================================================")
		fmt.Printf("  • Local UI & API : http://localhost:%s\n", a.cfg.Port)
		fmt.Printf("  • Bound Address  : %s\n", a.srv.Addr)
		fmt.Printf("  • Base Short URL : %s\n", a.cfg.BaseURL)
		fmt.Printf("  • Database (PG)  : %s\n", a.cfg.DatabaseURL)
		if a.cache != nil {
			fmt.Printf("  • Redis Cache    : %s (active)\n", a.cfg.RedisURL)
		} else {
			fmt.Println("  • Redis Cache    : disabled")
		}
		fmt.Println("==================================================")
		fmt.Println("Press Ctrl+C to shut down.")

		if err := a.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return err
	case <-quit:
		fmt.Println("\nShutting down server gracefully...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := a.srv.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown forced: %v", err)
			return err
		}
		fmt.Println("Server stopped.")
		return nil
	}
}

func (a *App) cleanup() {
	if a.cache != nil {
		_ = a.cache.Close()
	}
	if a.db != nil {
		_ = a.db.Close()
	}
}
