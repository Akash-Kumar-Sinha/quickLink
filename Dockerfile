FROM golang:1.26.1 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o quicklink ./cmd/server/main.go

FROM alpine:latest

RUN addgroup -S appuser && adduser -S appuser -G appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /app/quicklink ./quicklink

USER appuser

EXPOSE 8080

CMD ["./quicklink"]