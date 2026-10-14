# QuickLink

A minimalist URL shortener with Redis caching and an embedded single-page UI.

## Getting Started

1. Configure your environment in `.env`:
   ```env
   DATABASE_URL="postgresql://user:password@localhost:5432/dbname"
   REDIS_URL="redis://127.0.0.1:6379"
   ```

2. Run the server:
   ```bash
   go run ./cmd/server/main.go
   ```

The app will be available at `http://localhost:8080`.

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/` | Web UI |
| `POST` | `/api/shorten` | Shorten URL (`{"url": "..."}`) |
| `GET` | `/:code` | Redirect to target URL |
