# SchildCafé Servitør
# Copyright Carsten Thiel 2023
#
# SPDX-Identifier: Apache-2.0

# build the binary
FROM golang:latest AS builder
WORKDIR /go/src/servitor
COPY *.go /go/src/servitor/
COPY go.* /go/src/servitor/
COPY Makefile /go/src/servitor/
RUN make prep
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 make build

# package the binary into a container
FROM scratch
COPY --from=builder /go/src/servitor/servitor /servitor
CMD ["/servitor"]


