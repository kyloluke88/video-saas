FROM golang:1.24-alpine

WORKDIR /app

RUN apk add --no-cache git bash

COPY go.mod go.sum ./

RUN go mod download

RUN go install github.com/air-verse/air@v1.61.7

COPY . .

EXPOSE 8080

CMD ["air"]
