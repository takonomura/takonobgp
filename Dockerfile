FROM docker.io/library/golang:1.19.0-alpine AS build

WORKDIR /app

# 現時点で外部ライブラリ使ってないので
COPY go.mod ./
#COPY go.mod go.sum ./
#RUN go mod download

COPY *.go ./
RUN go build -o takonobgp .

FROM docker.io/library/alpine:latest

RUN apk add --no-cache curl

COPY --from=build /app/takonobgp /takonobgp

ENTRYPOINT ["/takonobgp"]
