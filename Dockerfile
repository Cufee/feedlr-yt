FROM golang:1.25-bookworm AS builder

WORKDIR /workspace

# Install Node.js and sqlite3 for builds
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && \
    apt-get install -y nodejs sqlite3

# Install atlas CLI for migrations
RUN curl -sSf https://atlasgo.sh | sh

# Install sqlboiler for model generation (aarondl fork)
RUN go install github.com/aarondl/sqlboiler/v4@latest && \
    go install github.com/aarondl/sqlboiler-sqlite3@latest

# Cache Go dependencies
COPY go.mod go.sum ./
RUN --mount=type=cache,target=$GOPATH/pkg/mod go mod download

# Cache npm dependencies
COPY package.json package-lock.json* ./
RUN npm ci || npm install

# Copy source code
COPY . ./

# Build Tailwind CSS
RUN npm run build

# Generate sqlboiler models
# Create temp database, apply migrations, then generate models
RUN sqlite3 /tmp/schema.db "" && \
    atlas migrate apply --allow-dirty \
    --dir "file://internal/database/migrations" \
    --url "sqlite:///tmp/schema.db?_fk=1" && \
    SQLITE3_DBNAME=/tmp/schema.db sqlboiler sqlite3

# Generate templ templates
RUN --mount=type=cache,target=$GOPATH/pkg/mod go generate ./...

# Build the final binary
RUN --mount=type=cache,target=$GOPATH/pkg/mod CGO_ENABLED=1 GOOS=linux go build -ldflags='-s -w' -trimpath -o app .

FROM debian:stable-slim

ENV TZ=Etc/UTC
ENV ZONEINFO=/zoneinfo.zip
COPY --from=builder /usr/local/go/lib/time/zoneinfo.zip /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=builder /workspace/app /usr/bin/app

ENTRYPOINT [ "app" ]
