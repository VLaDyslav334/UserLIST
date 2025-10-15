FROM golang:1.24.3 as builder

RUN go version
ENV GOPATH=/

WORKDIR /app

COPY go.mod go.sum ./

# install psql
RUN apt-get update
RUN apt-get -y install postgresql-client

## make wait-for-postgres.sh executable
#RUN chmod +x wait-for-postgres.sh

# build go app
RUN go mod download
RUN go build -o main ./cmd/

EXPOSE 8080

CMD ["./main"]
