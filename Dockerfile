FROM golang:1.21.1-alpine AS builder
WORKDIR /builder
COPY ./ /builder

RUN apk update && apk add \
    bash \
    iproute2 \
    iptables \
    gcc \
    libc-dev \
    libpcap-dev

RUN GOOS=linux CGO_ENABLED=1 GOARCH=amd64 go build -o ./deploy/bin/server ./deploy/server
RUN chmod +x ./deploy/bin/server
ENTRYPOINT ["./deploy/bin/server"]