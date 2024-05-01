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
COPY --from=assets /workspace/assets ./assets
COPY . ./

# add go modules lockfiles
RUN go mod download

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

COPY --from=build /workspace/app .

CMD ["./app"]