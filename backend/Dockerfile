# Go 后端镜像：多阶段构建，最终镜像只含静态二进制。
FROM golang:1.27-alpine AS build

WORKDIR /src
# 国内网络下 proxy.golang.org 不可达，优先走 goproxy.cn；海外机器可自行调整。
ENV GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata wget \
    && adduser -D -H -u 10001 app

COPY --from=build /out/server /usr/local/bin/server

USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]
