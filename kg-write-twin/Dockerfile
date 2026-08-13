# syntax=docker/dockerfile:1

# microcheck provides a tiny (~75KB) statically linked healthcheck binary, since
# the distroless runtime image below has no shell/curl. See:
# https://github.com/tarampampam/microcheck
FROM ghcr.io/tarampampam/microcheck:1@sha256:c9f79cd408626de7c10f2d487d67339f49adf0ba61dde96ede65343269db1f85 AS healthcheck

FROM golang:1.25-alpine@sha256:56961d79ea8129efddcc0b8643fd8a5416b4e6228cfd477e3fd61deb2672c587 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/kg-write-twin ./cmd/kg-write-twin

FROM gcr.io/distroless/static-debian12@sha256:61b7ccecebc7c474a531717de80a94709d20547cdcdaf740c25876f2a8e38b44
COPY --from=build /out/kg-write-twin /kg-write-twin
# Static healthcheck binary (no shell/curl in distroless).
COPY --from=healthcheck /bin/httpcheck /bin/httpcheck
# Port/base-path come from PORT/BASE_PATH env (see cmd/kg-write-twin/main.go), so
# the container can run on any port without rebuilding.
ENV PORT=8030 BASE_PATH=/api-server
EXPOSE 8030
# Probe the control-surface liveness endpoint.
HEALTHCHECK --interval=10s --timeout=5s --retries=5 \
    CMD ["/bin/httpcheck", "http://localhost:8030/api-server/_control/healthz"]
ENTRYPOINT ["/kg-write-twin"]
