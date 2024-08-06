FROM golang:1.22-bookworm as builder

WORKDIR /workspace

# install templ
RUN --mount=type=cache,target=$GOPATH/pkg/mod go install github.com/a-h/templ/cmd/templ@latest
# install task
RUN --mount=type=cache,target=$GOPATH/pkg/mod go install github.com/go-task/task/v3/cmd/task@latest
# install sqlboiler
RUN --mount=type=cache,target=$GOPATH/pkg/mod go install github.com/volatiletech/sqlboiler/v4@latest
RUN --mount=type=cache,target=$GOPATH/pkg/mod go install github.com/volatiletech/sqlboiler/v4/drivers/sqlboiler-sqlite3@latest 

# cache deps
COPY go.mod go.sum ./
RUN --mount=type=cache,target=$GOPATH/pkg/mod go mod download

COPY . ./

# generate code
RUN --mount=type=cache,target=$GOPATH/pkg/mod go generate ./internal/...

# build the final binary
RUN --mount=type=cache,target=$GOPATH/pkg/mod CGO_ENABLED=1 GOOS=linux go build -ldflags='-s -w' -trimpath -o app .

FROM debian:stable-slim

ENV TZ=Europe/Berlin
ENV ZONEINFO=/zoneinfo.zip
COPY --from=builder /usr/local/go/lib/time/zoneinfo.zip /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=builder /workspace/app /usr/bin/app

ENTRYPOINT [ "app" ]