FROM golang:1.13-alpine as build-env
ENV GO111MODULE=on

RUN mkdir /go/src/kvstore
WORKDIR /go/src/kvstore

COPY go.mod .
COPY go.sum .
RUN apk update && apk add git && apk add gcc
RUN go mod download
COPY kvstore .
RUN go build -o kvstore

FROM alpine:latest
WORKDIR /root/
COPY --from=build-env /go/src/kvstore/kvstore .

# Application is not initialized because application-level logging is
# enabled by a cmd line argument. 
#CMD ["./kvstore"]