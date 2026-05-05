# syntax=docker/dockerfile:1

ARG GO_VERSION=1.25.6

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build
WORKDIR /src

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download -x

ARG TARGETARCH

RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 GOARCH=$TARGETARCH go build -o /bin/server ./cmd

RUN --mount=type=bind,target=. \
    CGO_ENABLED=0 GOARCH=$TARGETARCH go build -o /bin/shard ./cmd/shard

FROM build AS shard
ARG SHARD_ID=0

RUN mkdir -p /out/resources

RUN --mount=type=bind,target=. \
    /bin/shard \
    -input resources/references.json.gz \
    -out /out/resources \
    -shard-id ${SHARD_ID} \
    -shard-count 2

FROM alpine:latest AS final

RUN --mount=type=cache,target=/var/cache/apk \
    apk --update add \
        ca-certificates \
        tzdata \
        wget \
        && \
        update-ca-certificates

ARG UID=10001

RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    appuser

COPY --from=build /bin/server /bin/server
COPY --from=shard /out/resources/references.vec /app/resources/references.vec
COPY --from=shard /out/resources/references.labels /app/resources/references.labels

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=5s --timeout=2s --start-period=10s --retries=3 \
  CMD wget --spider -q http://127.0.0.1:8080/ready || exit 1

ENTRYPOINT ["/bin/server"]
