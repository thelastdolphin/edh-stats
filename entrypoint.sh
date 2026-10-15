#!/bin/sh
set -e

echo "Starting edh-stats service..."
# Arguments are forwarded so that the image's HEALTHCHECK instruction (and
# `docker run <image> healthcheck`) can reach the binary's subcommands through
# the entrypoint as well.
exec ./edh-stats "$@"
