FROM golang:1.13-alpine as build-env
ENV GO111MODULE=on

RUN mkdir /go/src/client
WORKDIR /go/src/client

COPY go.mod /go/src/
COPY go.sum /go/src/

RUN apk update && apk add git && apk add gcc && apk add bash && apk add libc-dev

# dont need to download entire go.mod
RUN go get -u github.com/Lz-Gustavo/journey
RUN go get -u github.com/BurntSushi/toml

COPY scripts/genClients.sh .
COPY scripts/run.sh .
COPY client .

# used to debug local container on detach mode
#CMD tail -f /dev/null