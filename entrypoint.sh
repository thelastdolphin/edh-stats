#!/bin/sh
set -e

echo "Running db migrations..."
goose -dir ./migrations sqlite3 ./data/edh_stats.db up

echo "Starting edh-stats service..."
exec ./edh-stats