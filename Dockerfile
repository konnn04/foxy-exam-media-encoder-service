# ─── Build stage ────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o media-encoder-service .

# ─── Runtime stage ──────────────────────────────────────────────
FROM alpine:3.19

# FFmpeg is the only runtime dependency
RUN apk add --no-cache ffmpeg

WORKDIR /app
COPY --from=builder /app/media-encoder-service .

EXPOSE 8097

ENTRYPOINT ["./media-encoder-service"]
