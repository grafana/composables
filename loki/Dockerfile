# Custom Loki image with healthcheck support
# Extends official grafana/loki (distroless) by adding a static healthcheck binary

FROM ghcr.io/tarampampam/microcheck:1 AS healthcheck

FROM grafana/loki:latest

# Copy static healthcheck binary from microcheck image
# microcheck is a minimal (~75KB) statically linked healthcheck tool
# See: https://github.com/tarampampam/microcheck
COPY --from=healthcheck /bin/httpcheck /bin/httpcheck

# Healthcheck using the static binary
# Loki exposes /ready endpoint on port 3100
HEALTHCHECK --interval=10s --timeout=5s --retries=5 \
    CMD ["/bin/httpcheck", "http://localhost:3100/ready"]
