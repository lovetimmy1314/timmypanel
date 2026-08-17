# 容器化部署。日常不用手动构建这个文件——推 main 或推 v* tag 时
# .github/workflows/docker.yml 会自动构建多架构镜像推到
# ghcr.io/lovetimmy1314/timmypanel，VPS 上 `docker compose pull && up -d` 就行。
#
# 本地要自己构建（应急、或改了 Dockerfile 想先试）：
#
#   docker compose -f docker-compose.yml -f docker-compose.build.yml build
#   docker compose -f docker-compose.yml -f docker-compose.build.yml up -d
#
# 不想用 compose 就手敲（注意这些环境变量一个都不能少）：
#
#   docker build -f deploy/Dockerfile -t timmypanel .
#   docker run -d --name timmypanel -p 127.0.0.1:8080:8080 \
#     -v timmypanel-data:/data \
#     -e TP_SECURE=true -e TP_TRUSTED_PROXIES=172.17.0.1 timmypanel
#
# 前面有 HTTPS 反代时，这两项必须给：
#   TP_SECURE=true          会话 Cookie 才带 Secure 标记
#   TP_TRUSTED_PROXIES=...  反代过来的源地址（逗号分隔）。留空的话
#                           登录限流会被伪造的 X-Forwarded-For 绕过。
# 别用 TP_ADMIN_PASSWORD 设初始密码：applyEnv 跑在配置写回之前，
# 那个值会被明文持久化进卷里的 config.yaml。

# 前两个阶段都钉在 BUILDPLATFORM（构建机自身架构）上跑，**不跟着目标架构走**，
# 这样出多架构镜像时不用 QEMU 模拟整个编译过程：
#   - 前端产物是平台无关的 JS/CSS，一份就够，两个架构共用；
#   - Go 靠 CGO_ENABLED=0 + GOOS/GOARCH 直接交叉编译（和 build.ps1 同一招）。
# 只有最后那个 alpine 阶段是目标架构的，里面那几条 apk/adduser 才需要模拟。
FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend
WORKDIR /app
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# 前端产物必须在 go build 之前就位，否则 embed 会打进占位页面。
# 源路径的 /backend 不是笔误：frontend 阶段 WORKDIR 是 /app，而 vite 的
# outDir 是 ../backend/internal/web/dist，产物就落在容器的 /backend 下。
COPY --from=frontend /backend/internal/web/dist ./internal/web/dist
ARG VERSION=dev
# TARGETOS/TARGETARCH 由 buildx 按 --platform 自动注入，这里 ARG 一下是为了引进作用域。
# 单平台 docker build 也会有值（就是本机架构），所以两条路都不用特判。
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags "-s -w -X main.Version=${VERSION}" -o /out/timmypanel .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -H -u 10001 timmypanel
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=backend /out/timmypanel /app/timmypanel
# /data 必须在镜像里就归 10001。Docker 初始化空命名卷时会照搬镜像里该路径的
# 属主，VOLUME 自己建出来的是 root:root，那样首次启动写 /data/config.yaml 就失败。
RUN mkdir -p /data && chown 10001:10001 /data
USER timmypanel
VOLUME ["/data"]
EXPOSE 8080
ENV TP_LISTEN=0.0.0.0:8080 TP_DATA_DIR=/data
# /api/v1/auth/config 是唯一一个免登录的 GET（api/server.go），正好拿来探活。
# alpine 的 busybox 自带 wget，不用多装包。
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/api/v1/auth/config >/dev/null || exit 1
ENTRYPOINT ["/app/timmypanel", "-config", "/data/config.yaml"]
