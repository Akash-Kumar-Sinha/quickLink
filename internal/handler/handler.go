package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"quicklink/internal/shortener"
	"quicklink/internal/store"
	"quicklink/web"

	"github.com/gin-gonic/gin"
)

type Option func(*Handler)

func WithBaseURL(url string) Option {
	return func(h *Handler) {
		h.baseURL = strings.TrimRight(url, "/")
	}
}

func WithStore(s store.Store) Option {
	return func(h *Handler) {
		h.store = s
	}
}

func WithEngine(engine *gin.Engine) Option {
	return func(h *Handler) {
		h.engine = engine
	}
}

type Handler struct {
	baseURL string
	store   store.Store
	engine  *gin.Engine
}

type ShortenRequest struct {
	URL string `json:"url" binding:"required"`
}

func New(opts ...Option) *Handler {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(gin.Logger())
	engine.Use(corsMiddleware())

	h := &Handler{
		engine: engine,
	}

	for _, opt := range opts {
		opt(h)
	}

	h.registerRoutes()
	return h
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (h *Handler) registerRoutes() {
	h.engine.GET("/", h.handleIndex)
	h.engine.POST("/api/shorten", h.handleShorten)
	h.engine.GET("/:code", h.handleRedirect)
}

func (h *Handler) Engine() *gin.Engine {
	return h.engine
}

func (h *Handler) getBaseURL(c *gin.Context) string {
	if h.baseURL != "" {
		return h.baseURL
	}

	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}

func (h *Handler) handleIndex(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", web.IndexHTML)
}

func (h *Handler) handleShorten(c *gin.Context) {
	var req ShortenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	normURL, err := shortener.ValidateAndNormalizeURL(req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	var code string

	for attempts := 0; attempts < 5; attempts++ {
		randCode, err := shortener.GenerateRandomCode(shortener.DefaultCodeLength)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate short code"})
			return
		}

		if _, err := h.store.Get(ctx, randCode); errors.Is(err, store.ErrNotFound) {
			code = randCode
			break
		}
	}

	if code == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate unique code, please try again"})
		return
	}

	record := &store.URLRecord{
		Code:        code,
		OriginalURL: normURL,
		Clicks:      0,
		CreatedAt:   time.Now().UTC(),
	}

	if err := h.store.Save(ctx, record); err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "Short code or alias already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save URL: " + err.Error()})
		return
	}

	record.ShortURL = fmt.Sprintf("%s/%s", h.getBaseURL(c), code)
	c.JSON(http.StatusCreated, record)
}

func (h *Handler) handleRedirect(c *gin.Context) {
	code := c.Param("code")
	if code == "" || code == "favicon.ico" {
		c.Status(http.StatusNotFound)
		return
	}

	ctx := c.Request.Context()
	origURL, err := h.store.GetOriginalURL(ctx, code)
	if err != nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusNotFound, fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Link Not Found</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #ffffff; color: #000000; display: flex; align-items: center; justify-content: center; min-height: 100vh; margin: 0; }
    .card { border: 1px solid #000; padding: 32px; border-radius: 6px; text-align: center; max-width: 380px; }
    h1 { font-size: 2rem; margin: 0 0 8px; color: #dc2626; }
    p { color: #52525b; font-size: 0.9rem; margin-bottom: 20px; }
    a { display: inline-block; border: 1px solid #000; color: #000; padding: 8px 18px; border-radius: 4px; text-decoration: none; font-size: 0.85rem; font-weight: 500; }
    a:hover { background: rgba(0,0,0,0.05); }
  </style>
</head>
<body>
  <div class="card">
    <h1>404</h1>
    <h2>Link Not Found</h2>
    <p>The short link <code>/%s</code> does not exist.</p>
    <a href="/">Go to Homepage</a>
  </div>
</body>
</html>`, escapeHTML(code)))
		return
	}

	go func(targetCode string) {
		_ = h.store.IncrementClicks(context.Background(), targetCode)
	}(code)

	c.Redirect(http.StatusFound, origURL)
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
