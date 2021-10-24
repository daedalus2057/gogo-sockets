FROM golang:alpine AS builder

WORKDIR /build

ENV GOOS=linux \
  CGO_ENABLED=0 \
  GOARCH=amd64

COPY *.go .
COPY go.* .
COPY game ./game

RUN go build -o gogo-sockets .

EXPOSE 8080

FROM scratch

COPY --from=builder /build/gogo-sockets /

ENTRYPOINT ["/gogo-sockets"]





