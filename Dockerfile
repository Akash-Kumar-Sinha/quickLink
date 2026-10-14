FROM golang:1.26.1 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o quicklink ./cmd/server/main.go

FROM scratch

COPY --from=builder /app/quicklink /quicklink

EXPOSE 8080

CMD ["./quicklink"]