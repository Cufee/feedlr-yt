FROM node:22-bookworm AS styles-builder

WORKDIR /workspace

# cache frontend deps
COPY package.json package-lock.json tailwind.css ./
RUN npm ci

COPY . ./
RUN npm run build

FROM golang:1.25-bookworm AS builder

WORKDIR /workspace

# cache deps
COPY go.mod go.sum ./
RUN --mount=type=cache,target=$GOPATH/pkg/mod go mod download

COPY . ./
COPY --from=styles-builder /workspace/assets/css/style.css /workspace/assets/css/style.css

# generate code
RUN --mount=type=cache,target=$GOPATH/pkg/mod go generate ./...

# build the final binary
RUN --mount=type=cache,target=$GOPATH/pkg/mod CGO_ENABLED=1 GOOS=linux go build -ldflags='-s -w' -trimpath -o app .

FROM debian:stable-slim

ENV TZ=Etc/UTC
ENV ZONEINFO=/zoneinfo.zip
COPY --from=builder /usr/local/go/lib/time/zoneinfo.zip /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=builder /workspace/app /usr/bin/app

ENTRYPOINT [ "app" ]
