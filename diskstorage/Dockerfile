FROM golang:1.13-alpine as build-env
ENV GO111MODULE=on

RUN mkdir /go/src/diskstorage
WORKDIR /go/src/diskstorage

COPY go.mod .
COPY go.sum .
RUN apk update && apk add git && apk add gcc
RUN go mod download
COPY diskstorage .
RUN go build -o diskstorage

FROM alpine:latest
WORKDIR /root/
COPY --from=build-env /go/src/diskstorage/diskstorage .
