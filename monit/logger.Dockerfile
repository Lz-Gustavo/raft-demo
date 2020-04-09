# Builds the logger process binary with monit_sys

FROM golang:1.13 as build-env
ENV GO111MODULE=on

RUN mkdir /go/src/logger
WORKDIR /go/src/logger

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY logger .
RUN go build -o logger

FROM python:3.7-slim
RUN mkdir /monit
WORKDIR /monit

COPY --from=build-env /go/src/logger/logger .

COPY monit .

RUN apt-get update 
RUN yes | apt-get install gcc

RUN pip3 install -r requirements.txt --upgrade
