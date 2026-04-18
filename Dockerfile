# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM golang:1.26-alpine3.23 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY cmd ./cmd

ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-w -s" -o /out/alert-forwarder ./cmd/alert-forwarder

FROM alpine:3.23

RUN apk --no-cache add ca-certificates tzdata \
 && addgroup -S app \
 && adduser -S -G app app

COPY --from=builder /out/alert-forwarder /usr/local/bin/alert-forwarder

USER app:app
EXPOSE 8888

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8888/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/alert-forwarder"]
