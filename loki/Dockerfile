# Custom Loki image with healthcheck support
# Extends official grafana/loki (distroless) by adding a static healthcheck binary

FROM ghcr.io/tarampampam/microcheck:1@sha256:c9f79cd408626de7c10f2d487d67339f49adf0ba61dde96ede65343269db1f85 AS healthcheck

FROM grafana/loki:latest@sha256:73e905b51a7f917f7a1075e4be68759df30226e03dcb3cd2213b989cc0dc8eb4

# Copy static healthcheck binary from microcheck image
# microcheck is a minimal (~75KB) statically linked healthcheck tool
# See: https://github.com/tarampampam/microcheck
COPY --from=healthcheck /bin/httpcheck /bin/httpcheck

# Healthcheck using the static binary
# Loki exposes /ready endpoint on port 3100
HEALTHCHECK --interval=10s --timeout=5s --retries=5 \
    CMD ["/bin/httpcheck", "http://localhost:3100/ready"]
