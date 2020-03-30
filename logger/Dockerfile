FROM golang:1.13-alpine as build-env
ENV GO111MODULE=on

RUN mkdir /go/src/logger
WORKDIR /go/src/logger

COPY go.mod .
COPY go.sum .
RUN apk update && apk add git && apk add gcc
RUN go mod download
COPY logger .
RUN go build -o logger

FROM alpine:latest
WORKDIR /root/
COPY --from=build-env /go/src/logger/logger .
