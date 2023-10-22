FROM golang:1.20 as build

WORKDIR /workspace

# install node
RUN apt update && apt install nodejs npm -y

# add go modules lockfiles
COPY go.mod go.sum ./
RUN go mod download

# prefetch the binaries
COPY ./prisma/download.go ./prisma/download.go
ENV PRISMA_CLI_QUERY_ENGINE_BINARY=/workspace/prisma/bin/engine
RUN mv `go run prisma/download.go` $PRISMA_CLI_QUERY_ENGINE_BINARY
RUN go run github.com/steebchen/prisma-client-go prefetch

# install templ
RUN go install github.com/a-h/templ/cmd/templ@latest
# install task
RUN go install github.com/go-task/task/v3/cmd/task@latest

COPY . ./

# generate the Prisma Client Go client
RUN task build

FROM scratch as bin

ENV PRISMA_CLI_QUERY_ENGINE_BINARY=/prisma/bin/engine

COPY --from=build /workspace/app /app
COPY --from=build /workspace/assets /assets
COPY --from=build /workspace/prisma/bin /prisma/bin

CMD ["/app"]