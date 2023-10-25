FROM node as assets

WORKDIR /workspace
COPY . ./

RUN npm install && npm install -g @go-task/cli
RUN task style:generate


FROM golang:1.20 as build

WORKDIR /workspace
COPY . ./

# add go modules lockfiles
RUN go mod download

# download the engine required for the next image
RUN go run prisma/download.go

# install templ
RUN go install github.com/a-h/templ/cmd/templ@latest
# install task
RUN go install github.com/go-task/task/v3/cmd/task@latest

# generate the Prisma Client Go client
RUN task build:docker

FROM scratch as run

ENV TZ=Europe/Berlin
ENV ZONEINFO=/zoneinfo.zip
COPY --from=build /usr/local/go/lib/time/zoneinfo.zip /
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

ENV PRISMA_QUERY_ENGINE_BINARY=/prisma/bin/engine
COPY --from=build /workspace/prisma/bin/engine /prisma/bin/engine

COPY --from=build /workspace/app .

CMD ["./app"]