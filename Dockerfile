FROM golang:1.20 as build

WORKDIR /workspace

# install node
RUN apt update && apt install nodejs npm -y

# add go modules lockfiles
COPY go.mod go.sum ./
RUN go mod download

# prefetch the binaries
RUN go run github.com/steebchen/prisma-client-go prefetch

# install templ
RUN go install github.com/a-h/templ/cmd/templ@latest
# install task
RUN go install github.com/go-task/task/v3/cmd/task@latest

COPY . ./

# We need a git repo in order to get a commit during build, the commit ID itself does not really matter though
RUN git config --global init.defaultBranch main && \
  git config --global user.email "pipeline@byvko.dev" && \
  git config --global user.name "Docker Build" && \
  git init && \
  git add . && \
  git commit -m "build commit"

# generate the Prisma Client Go client
RUN task build:full

CMD ["./app"]