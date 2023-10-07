FROM golang:1.20-buster as builder

WORKDIR /workspace

# add go modules lockfiles
COPY go.mod go.sum ./
RUN go mod download

# prefetch the binaries, so that they will be cached and not downloaded on each change
RUN go run github.com/steebchen/prisma-client-go prefetch

COPY ./ ./
# generate the Prisma Client Go client
RUN go generate ./...

# build a fully standalone binary with zero dependencies
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o app .

# use the scratch image for the smallest possible image size
FROM scratch

# copy over SSL certificates, so that we can make HTTPS requests
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=builder /workspace/app /app

ENTRYPOINT ["/app"]