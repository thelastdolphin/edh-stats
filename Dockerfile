# The image is *assembled* here from a binary that was compiled outside, in a plain
# container (CI: the build-binary step; locally: `make image-arm64` / `image-amd64`,
# which compile first).
#
# Compiling inside the image was tried and deliberately reverted. The host that
# builds and runs this service is a 4 GB Raspberry Pi 4, where the CI plugin wraps
# every build in its own nested dockerd: a full cold standard-library build in that
# environment (four package compiles, an apt-installed cross toolchain, a registry
# cache export) is far more than that machine handles reliably, while the previous
# shape - compile in a plain cached step, then assemble - demonstrably worked there.
#
# There are no RUN steps below, which also means an image for another architecture
# can be assembled without emulation.

FROM debian:trixie-slim

WORKDIR /app

COPY --chmod=0755 edh-stats /app/edh-stats
COPY --chmod=0755 entrypoint.sh /app/entrypoint.sh

# The data directory is taken from the build context purely to get the ownership
# right without a RUN step: a fresh named volume, and a plain `docker run` with no
# mount at all, are then writable by the runtime uid.
COPY --chown=10001:10001 packaging/data /app/data

# Numeric uid/gid on purpose: docker-compose overrides this with the uid that owns
# the bind-mounted data directory.
USER 10001:10001

EXPOSE 8080

# The probe is the service binary itself, so the image needs no HTTP client and no
# extra package. It asks the real /health endpoint, which pings the database, so
# "healthy" means the service is up and has wired everything together.
# start-period covers the first startup, when migrations are applied.
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD ["/app/edh-stats", "healthcheck"]

ENTRYPOINT ["/app/entrypoint.sh"]
