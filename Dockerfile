FROM node:26.12.1-bookworm AS frontend-builder

WORKDIR /workspace

# cache frontend deps
COPY package.json package-lock.json ./
RUN npm ci
COPY web/package.json web/package-lock.json ./web/
RUN npm --prefix web ci

COPY . ./
RUN npm run build

FROM golang:1.27.3-bookworm AS builder

WORKDIR /workspace

# cache deps
COPY go.mod go.sum ./
RUN --mount=type=cache,target=$GOPATH/pkg/mod go mod download

COPY . ./
COPY --from=frontend-builder /workspace/assets /workspace/assets

# build the final binary
RUN --mount=type=cache,target=$GOPATH/pkg/mod CGO_ENABLED=1 GOOS=linux go build -ldflags='-s -w' -trimpath -o app .

FROM debian:stable-slim

RUN apt-get update && apt-get install -y --no-install-recommends ffmpeg && rm -rf /var/lib/apt/lists/*

ENV TZ=Etc/UTC
ENV ZONEINFO=/zoneinfo.zip
COPY --from=builder /usr/local/go/lib/time/zoneinfo.zip /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=builder /workspace/app /usr/bin/app

ENTRYPOINT [ "app" ]
