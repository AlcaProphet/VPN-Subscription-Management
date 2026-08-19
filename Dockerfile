# ============ 阶段一：前端构建 ============
FROM node:22-alpine AS frontend
WORKDIR /build
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ============ 阶段二：后端静态编译（CGO_ENABLED=0）============
FROM golang:1.26-alpine AS backend
WORKDIR /build
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# 用真实前端产物替换 embed 占位目录，再编译进二进制
COPY --from=frontend /build/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ============ 阶段三：最小运行时 ============
FROM alpine:3.21
# alpine 自带 wget/shell，healthcheck 可用；创建非 root 用户运行（Design1 §7.1）
RUN addgroup -S app && adduser -S app -G app
COPY --from=backend /out/server /server
# 预建 /public 目录结构（安装包/站点资源，Build2 填充内容）；数据卷承载全部持久化
ENV DATA_DIR=/data
RUN mkdir -p /data && chown -R app:app /data
VOLUME ["/data"]
EXPOSE 8080
USER app
ENTRYPOINT ["/server"]
