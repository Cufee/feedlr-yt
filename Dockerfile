FROM node:21 as assets

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


FROM golang:1.22-alpine as build

WORKDIR /workspace

# install templ
RUN go install github.com/a-h/templ/cmd/templ@latest
# install task
RUN go install github.com/go-task/task/v3/cmd/task@latest
# install sqlboiler
RUN go install github.com/volatiletech/sqlboiler/v4@latest
RUN go install github.com/volatiletech/sqlboiler/v4/drivers/sqlboiler-sqlite3@latest 

COPY go.mod go.sum ./
RUN --mount=type=cache,target=$GOPATH/pkg/mod go mod download

COPY --from=assets /workspace/assets ./assets
COPY . ./

# generate code
RUN --mount=type=cache,target=$GOPATH/pkg/mod go generate ./internal/...

# build the final binary
RUN --mount=type=cache,target=$GOPATH/pkg/mod go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o app .

FROM scratch as run

ENV TZ=Europe/Berlin
ENV ZONEINFO=/zoneinfo.zip
COPY --from=build /usr/local/go/lib/time/zoneinfo.zip /
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

COPY --from=build /workspace/app .

CMD ["./app"]