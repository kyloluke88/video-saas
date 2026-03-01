FROM golang:1.24-alpine

RUN apk add --no-cache \
    git \
    ffmpeg \
    fontconfig \
    font-noto-cjk \
    ttf-dejavu

WORKDIR /app

RUN go install github.com/air-verse/air@v1.61.7

COPY go.mod go.sum ./
RUN go mod download

COPY . .

CMD ["air"]
