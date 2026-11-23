# syntax=docker/dockerfile:1
# Messora backend: one static binary serving every role (api, worker,
# scheduler, migrate, init). Runtime image: distroless, non-root, no shell.

FROM golang:1.27.4@sha256:205c7636d74e0ffbe3d007162df7d74f859f5a8c8322dfb9f907977686c126cf AS build
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
      -o /out/messora ./cmd/messora \
 && mkdir -p /out/media

FROM gcr.io/distroless/static-debian13:nonroot@sha256:8ce6dd87e9823c5b48dcfb2c8604714ccb351ce59607327bcc4957448da279b5
COPY --from=build /out/messora /usr/local/bin/messora
# A fresh named volume mounted here inherits this ownership.
COPY --from=build --chown=65532:65532 /out/media /var/lib/messora/media
USER nonroot:nonroot
EXPOSE 8080 9100
ENTRYPOINT ["/usr/local/bin/messora"]
CMD ["help"]
