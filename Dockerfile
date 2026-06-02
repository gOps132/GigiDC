# syntax=docker/dockerfile:1

FROM golang:1.26.3-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
ARG TARGETOS
ARG TARGETARCH

RUN target_os="${TARGETOS:-$(go env GOOS)}"; \
    target_arch="${TARGETARCH:-$(go env GOARCH)}"; \
    CGO_ENABLED=0 GOOS="${target_os}" GOARCH="${target_arch}" go build \
      -ldflags "-s -w -X github.com/gOps132/GigiDC/internal/buildinfo.Version=${VERSION} -X github.com/gOps132/GigiDC/internal/buildinfo.Commit=${COMMIT} -X github.com/gOps132/GigiDC/internal/buildinfo.BuildTime=${BUILD_TIME}" \
      -o /out/gigi ./cmd/gigi

FROM alpine:3.22

RUN adduser -D -H -u 10001 gigi
USER gigi
WORKDIR /app
COPY --from=build /out/gigi /app/gigi

EXPOSE 8080
ENTRYPOINT ["/app/gigi"]
