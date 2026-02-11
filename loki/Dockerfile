# Custom Loki image with healthcheck support
# Extends official grafana/loki (distroless) by adding a static healthcheck binary

FROM ghcr.io/tarampampam/microcheck:1@sha256:79c187c05bfa67518078bf4db117771942fa8fe107dc79a905861c75ddf28dfa AS healthcheck

FROM grafana/loki:latest@sha256:847c287ada0e12603910589f42038c5cdaaad04e248bd1dc6c6e0920a235f427

# Copy static healthcheck binary from microcheck image
# microcheck is a minimal (~75KB) statically linked healthcheck tool
# See: https://github.com/tarampampam/microcheck
COPY --from=healthcheck /bin/httpcheck /bin/httpcheck

# Healthcheck using the static binary
# Loki exposes /ready endpoint on port 3100
HEALTHCHECK --interval=10s --timeout=5s --retries=5 \
    CMD ["/bin/httpcheck", "http://localhost:3100/ready"]
