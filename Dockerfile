FROM golang:1.23-bookworm as builder

WORKDIR /workspace

# install sqlboiler
RUN --mount=type=cache,target=$GOPATH/pkg/mod go install github.com/aarondl/sqlboiler/v4@latest
RUN --mount=type=cache,target=$GOPATH/pkg/mod go install github.com/aarondl/sqlboiler/v4/drivers/sqlboiler-sqlite3@latest

# cache deps
COPY go.mod go.sum ./
RUN --mount=type=cache,target=$GOPATH/pkg/mod go mod download

COPY . ./

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
