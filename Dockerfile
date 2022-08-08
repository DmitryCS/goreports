FROM golang:1.18.1-alpine3.15 as build

RUN apk add g++ make cmake sudo curl pkgconfig linux-headers

COPY go.sum /app/
COPY go.mod /app/

WORKDIR /app

RUN go mod download
RUN cd $GOPATH/pkg/mod/gocv.io/x/gocv@v0.31.0 && make install

COPY ./config /app/config
COPY ./main.go /app/

RUN go build -o main .
RUN rm -rf /usr/local/lib/cmake /usr/local/lib/pkgconfig
FROM alpine:3.15.4

LABEL maintainer="CUBICAI"

ENV DEBIAN_FRONTEND=noninteractive

RUN apk add libstdc++ && rm -rf /var/cache/apk/*
COPY --from=build /usr/local/lib /usr/local/lib
COPY --from=build /app/main /app/main

ENTRYPOINT ["/app/main"]
