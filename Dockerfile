# 适配 Glibc 动态链接与 CGO
FROM golang:1.24-bookworm AS builder

# 安装 CGO 编译工具链
RUN apt-get update -o Acquire::Check-Valid-Until=false \
    && apt-get install -y gcc g++ musl-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
ENV GOTOOLCHAIN=auto
ENV GOPROXY=https://goproxy.cn,direct

COPY go.mod go.sum ./
RUN go mod tidy
COPY . .

RUN CGO_CFLAGS="-Os -g0" CGO_ENABLED=1 \
    go build -trimpath \
    -ldflags="-s -w" \
    -o /app/submatrix .

# 基础镜像使用带有 glibc 的 busybox
FROM busybox:glibc

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo/Asia/Shanghai /etc/localtime
COPY --from=builder /usr/share/zoneinfo/Asia/Shanghai /usr/share/zoneinfo/Asia/Shanghai

WORKDIR /app
ENV TZ=Asia/Shanghai
ENV RUNNING_IN_DOCKER=true

LABEL org.opencontainers.image.title="SubMatrix" \
    org.opencontainers.image.description="高性能订阅拓扑矩阵与检测筛选工具"

COPY --from=builder /app/submatrix /app/submatrix

EXPOSE 8199
CMD ["/app/submatrix"]
