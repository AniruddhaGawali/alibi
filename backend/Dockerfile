# syntax=docker/dockerfile:1
#
# One build, two deployable services (build with --target):
#   api     the HTTP API (last stage, so it is the default for a plain build).
#           Set EMBED_EMAIL_WORKER=true to also run the email worker in it.
#   worker  the email worker (+ the emailctl admin tool)
# Everything is configured through environment variables (see ../.env.example).

FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0
# nodynamic: gen2brain/webp otherwise links against the glibc loader (via purego) even with
# CGO off, and the binary then fails on alpine (musl) with "no such file or directory".
RUN go build -tags nodynamic -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && go build -tags nodynamic -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
 && go build -tags nodynamic -trimpath -ldflags="-s -w" -o /out/emailctl ./cmd/emailctl

FROM alpine:3.20 AS base
RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 app
USER app

FROM base AS worker
COPY --from=build /out/worker /out/emailctl /usr/local/bin/
EXPOSE 8081
ENTRYPOINT ["worker"]

FROM base AS api
COPY --from=build /out/api /usr/local/bin/api
EXPOSE 8080
ENTRYPOINT ["api"]
