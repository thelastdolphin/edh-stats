FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates sqlite3 curl net-tools && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY edh-stats .
COPY migrations/sqlite3 ./migrations
COPY entrypoint.sh .

RUN adduser --disabled-password --gecos '' appuser && \
    chown -R appuser:appuser /app
USER appuser

EXPOSE 8080

ENTRYPOINT ["/app/entrypoint.sh"]
