FROM golang:alpine
# 为我们的镜像设置必要的环境变量
ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOPROXY="https://proxy.golang.com.cn,direct"
# 移动到工作目录：/build
WORKDIR /build
COPY . .
RUN go build cmd/rec53.go

WORKDIR /dist
RUN cp /build/rec53 .

EXPOSE 5353 9999

ENTRYPOINT ["/dist/rec53"]