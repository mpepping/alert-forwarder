FROM golang:1.14-alpine

COPY ./cmd /go/src/alert-forwarder/cmd
COPY ./go.mod go.sum /go/src/alert-forwarder

WORKDIR /go/src/alert-forwarder

RUN CGO_ENABLED=0 GOOS=linux go install -ldflags="-w -s" -v ./cmd/alert-forwarder/...

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=0 /go/bin/alert-forwarder /bin/alert-forwarder
CMD ["/bin/alert-forwarder"]
