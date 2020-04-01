FROM golang:1.10-alpine

WORKDIR /go/src/

COPY . kvstore/

RUN apk update
RUN apk add git
RUN go get -u github.com/Lz-Gustavo/journey
RUN go get -u github.com/hashicorp/raft
RUN go get -u github.com/hashicorp/go-hclog
RUN cd kvstore/ && go install