# Builds diskstorage binary with monit_sys

FROM golang:1.13 as build-env
ENV GO111MODULE=on

RUN mkdir /go/src/diskstorage
WORKDIR /go/src/diskstorage

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY diskstorage .
RUN go build -o diskstorage

FROM python:3.7-slim
RUN mkdir /monit
WORKDIR /monit

COPY --from=build-env /go/src/diskstorage/diskstorage .

COPY monit .

RUN apt-get update 
RUN yes | apt-get install gcc

RUN pip3 install -r requirements.txt --upgrade

#CMD ["./diskstorage"]
#CMD ["./diskstorage -logfolder=/tmp/"]
