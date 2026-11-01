# 适配 Glibc 动态链接与 CGO
FROM golang:1.24-bookworm AS builder

RUN apt-get update -o Acquire::Check-Valid-Until=false \
    && apt-get install -y wget xz-utils gcc g++ musl-dev \
    && wget https://github.com/upx/upx/releases/download/v4.2.2/upx-4.2.2-amd64_linux.tar.xz \
    && tar -xf upx-4.2.2-amd64_linux.tar.xz \
    && mv upx-4.2.2-amd64_linux/upx /usr/local/bin/upx \
    && rm -rf upx-4.2.2-amd64_linux*

WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum ./
RUN go mod tidy
COPY . .

RUN CGO_CFLAGS="-Os -g0" CGO_ENABLED=1 \
    go build -trimpath \
    -ldflags="-s -w" \
    -o /app/submatrix .

# 执行 UPX 压缩
RUN upx -5 /app/submatrix || true

# 基础镜像使用带有 glibc 的 busybox
FROM busybox:glibc

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
COPY --from=builder /usr/share/zoneinfo/Asia/Shanghai /usr/share/zoneinfo/Asia/Shanghai

WORKDIR /app
ENV TZ=Asia/Shanghai
ENV RUNNING_IN_DOCKER=true

LABEL org.opencontainers.image.title="SubMatrix" \
    org.opencontainers.image.description="高性能订阅拓扑矩阵与检测聚合平台"

COPY --from=builder /app/submatrix /app/submatrix

EXPOSE 8199
CMD ["/app/submatrix"]
