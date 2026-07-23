# Stage 1: Build frontend
FROM node:22-alpine AS frontend-builder
WORKDIR /app
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ .
RUN npm run build

# Stage 2: Build Go binary
FROM golang:alpine AS backend-builder
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o server ./cmd/server

# Stage 3: Runtime (distroless, zero CGO)
FROM gcr.io/distroless/static-debian12
COPY --from=backend-builder /app/server /server
COPY --from=frontend-builder /app/dist /app/public
ENV DATA_DIR=/app/data
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/server"]
