FROM node as assets

WORKDIR /workspace
COPY . ./

# We need a git repo in order to get a commit during build, the commit ID itself does not really matter though
RUN git config --global init.defaultBranch main && \
  git config --global user.email "pipeline@byvko.dev" && \
  git config --global user.name "Docker Build" && \
  git init && \
  git add . && \
  git commit -m "build commit"

RUN npm install && npm install -g @go-task/cli
RUN task style:generate


FROM golang:1.20 as build

WORKDIR /workspace
COPY . ./

# install node
RUN apt update && apt install nodejs npm -y

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
COPY --from=assets /workspace/assets ./assets

CMD ["./app"]