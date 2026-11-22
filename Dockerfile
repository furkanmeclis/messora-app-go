# syntax=docker/dockerfile:1
# Messora backend: one static binary serving every role (api, worker,
# scheduler, migrate, init). Runtime image: distroless, non-root, no shell.

FROM golang:1.27.4 AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOTOOLCHAIN=local GOFLAGS=-mod=readonly
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath \
      -ldflags "-s -w -X github.com/furkanmeclis/messora-app-go/internal/app.Version=${VERSION}" \
      -o /out/messora ./cmd/messora

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/messora /usr/local/bin/messora
USER nonroot:nonroot
EXPOSE 8080 9100
ENTRYPOINT ["/usr/local/bin/messora"]
CMD ["help"]
