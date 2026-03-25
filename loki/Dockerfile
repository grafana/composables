# Custom Loki image with healthcheck support
# Extends official grafana/loki (distroless) by adding a static healthcheck binary

FROM ghcr.io/tarampampam/microcheck:1@sha256:79c187c05bfa67518078bf4db117771942fa8fe107dc79a905861c75ddf28dfa AS healthcheck

FROM grafana/loki:latest@sha256:9db961170b729dc38277963cbb4a79941626388562cc455db61bf85564aa8945

# Copy static healthcheck binary from microcheck image
# microcheck is a minimal (~75KB) statically linked healthcheck tool
# See: https://github.com/tarampampam/microcheck
COPY --from=healthcheck /bin/httpcheck /bin/httpcheck

# Healthcheck using the static binary
# Loki exposes /ready endpoint on port 3100
HEALTHCHECK --interval=10s --timeout=5s --retries=5 \
    CMD ["/bin/httpcheck", "http://localhost:3100/ready"]
