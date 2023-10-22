FROM golang:1.20 as build

WORKDIR /workspace

# add go modules lockfiles
COPY go.mod go.sum ./
RUN go mod download

# prefetch the binaries, so that they will be cached and not downloaded on each change
RUN go run github.com/steebchen/prisma-client-go prefetch

# install templ
RUN go install github.com/a-h/templ/cmd/templ@latest

COPY . ./

# generate the Prisma Client Go client
RUN go generate ./...

# build the binary with all dependencies
RUN go build -o /app .

CMD ["/app"]