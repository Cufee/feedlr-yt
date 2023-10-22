FROM golang:1.20 as build

WORKDIR /workspace

# install node
RUN apt update && apt install nodejs npm -y

# add go modules lockfiles
COPY go.mod go.sum ./
RUN go mod download

# prefetch the binaries, so that they will be cached and not downloaded on each change
RUN go run github.com/steebchen/prisma-client-go prefetch

# install templ
RUN go install github.com/a-h/templ/cmd/templ@latest
# install task
RUN go install github.com/go-task/task/v3/cmd/task@latest

COPY . ./

# generate the Prisma Client Go client
RUN task build

CMD ["/app"]