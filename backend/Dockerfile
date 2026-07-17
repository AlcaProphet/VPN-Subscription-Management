# Stage 1: Build
FROM golang:alpine AS builder
WORKDIR /app

# Install build dependencies (modernc.org/sqlite is pure Go, no CGO needed)
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o server ./cmd/server

# Stage 2: Runtime
FROM gcr.io/distroless/static-debian12
COPY --from=builder /app/server /server
ENV DATA_DIR=/app/data
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/server"]
