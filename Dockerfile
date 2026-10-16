# Base images pinned by multi-arch index digest (resolved 2026-10-17).
ARG GO_IMAGE=golang:1.26.9-alpine@sha256:3082400e369fa24d5fc60bca20edab3f6d604e0c5a690ec66b295eff4dd87ade
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3

# Cross-compile on the host platform: no QEMU needed for arm64/amd64 targets.
FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS build
WORKDIR /src
ENV CGO_ENABLED=0 \
    GOTOOLCHAIN=local \
    GOFLAGS=-mod=readonly

# Dependencies first: this layer is reused until go.mod/go.sum change.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=bind,source=go.mod,target=go.mod \
    --mount=type=bind,source=go.sum,target=go.sum \
    go mod download && go mod verify

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -buildvcs=false \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/server ./cmd/server

# docker build --target test .
FROM build AS test
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    go vet ./... && go test -count=1 ./...

FROM ${RUNTIME_IMAGE} AS runtime
COPY --from=build /out/server /app/server
USER 65532:65532
ENV HTTP_ADDR=:8080
EXPOSE 8080
# distroless has no shell: the binary probes its own /readyz.
HEALTHCHECK --interval=5s --timeout=3s --start-period=30s --retries=3 \
  CMD ["/app/server", "healthcheck"]
ENTRYPOINT ["/app/server"]
CMD ["serve"]
