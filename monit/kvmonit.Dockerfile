# Builds kvstore binary with monit_sys

FROM golang:1.13 as build-env
ENV GO111MODULE=on

RUN mkdir /go/src/kvstore
WORKDIR /go/src/kvstore

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY kvstore .
RUN go build -o kvstore

FROM python:3.7-slim
RUN mkdir /monit
WORKDIR /monit

COPY --from=build-env /go/src/kvstore/kvstore .

COPY monit .

RUN apt-get update 
RUN yes | apt-get install gcc

RUN pip3 install -r requirements.txt --upgrade

#CMD ["./kvstore"]
#CMD ["./kvstore -logfolder=/tmp/"]
