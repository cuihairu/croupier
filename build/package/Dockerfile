FROM golang:bullseye as builder

ARG GITCOMMIT=""

ENV GO111MODULE=on \
    GOPROXY=https://goproxy.cn,direct \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /app

COPY . .

RUN mkdir -p dist/bin  && go build -a -ldflags "-s -X github.com/chuihairu/croupier/internal/version.GitCommit=${GITCOMMIT}" -o dist/bin/server github.com/chuihairu/croupier/cmd/server


FROM nginx:stable

WORKDIR /app

COPY --from=builder /app/dist  .

# 指定运行时环境变量
ENV GIN_MODE=release \
    PORT=80

EXPOSE 80

ENTRYPOINT ["./bin/server"]