# edh-stats

Small app handling statistics for EDH games.

Design document (in Russian): https://docs.google.com/document/d/18G8sqmAyTxzO-AKIA15zp99Wp6BqGOQbevd1uOmMs3Y/edit

Written in Golang 1.27.

## Configuration

The service is configured through the environment:

| Variable         | Default                 | Meaning                          |
| ---------------- | ----------------------- | -------------------------------- |
| `DB_PATH`        | `./data/edh_stats.db`   | SQLite database file.            |
| `SERVER_ADDRESS` | `:8080`                 | Address the HTTP server binds to.|

Schema migrations are embedded into the binary and applied automatically at
startup, so a fresh database needs no manual preparation.

## API

| Method | Path          | Description                                                                                                                                                                    |
| ------ | ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `POST` | `/decks`      | Creates a deck. Body: `{"name": "...", "owner": "...", "type": "..."}`. Every field is required and limited to 50 characters; `type` is free text. `201` with a `Location` header, `400` for invalid input, `409` when the name+owner pair already exists. |
| `GET`  | `/decks`      | Lists decks. Query parameters: `name`, `owner`, `type`, `page` (from 1, default 1), `limit` (1–100, default 20). Returns `[]` when nothing matches.                              |
| `GET`  | `/decks/{id}` | Returns one deck, or `404` when it does not exist.                                                                                                                             |
| `PATCH` | `/decks/{id}` | Updates the given fields of a deck: `name`, `owner`, `type`. Only the fields present in the body are changed. `200` with the updated deck, `400` for invalid input, `404` when it does not exist, `409` when the resulting name+owner pair is already taken. |
| `DELETE` | `/decks/{id}` | Deletes a deck: `204` on success, `404` when it does not exist, `409` when a game still refers to it.                                                                                    |
| `POST` | `/games`      | Records a game. Body: `{"played_at":"2026-02-07","notes":"...","decks":[{"deck_id":1,"player":"...","result":"win"},{"deck_id":2}]}`. The first deck is the one recording the game and only its result is stored; `played_at` defaults to today and `result` is optional and must be `win`, `loss` or `draw`. `201` with a `Location` header, `400` for invalid input or an unknown deck. |
| `GET`  | `/games`      | Lists games, newest first, each with its participants. Query parameters: `deck_id`, `player`, `from`, `to` (inclusive `YYYY-MM-DD`), `page`, `limit`. Returns `[]` when nothing matches. |
| `GET`  | `/games/{id}` | Returns one game with its participants, or `404` when it does not exist. |
| `DELETE` | `/games/{id}` | Deletes a game together with its participants: `204` on success, `404` when it does not exist. |
| `GET`  | `/health`     | `200` while the database answers, `503` otherwise.                                                                                                                             |

Request bodies are limited to 1 MiB. An unknown path answers `404`, and a known
path with the wrong method answers `405` (with an `Allow` header).

Errors are JSON objects with a machine-readable code and a message meant for a human:

```json
{"error": {"code": "deck_not_found", "message": "deck not found"}}
```

The codes are `invalid_id`, `invalid_query`, `invalid_json`, `body_too_large`,
`validation_failed`, `deck_not_found`, `game_not_found`, `deck_duplicate`,
`deck_in_use`, `unknown_deck`, `duplicate_participant` and `internal_error`. A client
reacts to the code; the message explains the problem and may be reworded at any time.

## Development

```sh
make build      # native binary
make test       # go test -count=1 (portable; this is what CI runs)
make test-race  # go test -race, where ThreadSanitizer is supported
make check      # vet + test
make run        # go run ./cmd/edh-stats
make build-arm64  # cross-compiled Linux/arm64 binary (see the Makefile for the toolchain package)
```

## Container images

The `Dockerfile` only *assembles* the image: it copies `edh-stats` and
`entrypoint.sh` into the runtime stage and contains no `RUN` steps at all. The
binary is compiled outside, in a plain step against the persistent Go cache - which
is the shape that works on the 4 GB Raspberry Pi that builds and runs this service.
Compiling inside the image (a cold standard-library build inside the CI plugin's
nested dockerd) was tried and deliberately reverted.

```sh
make image-arm64   # compiles for arm64, then assembles the image
make image-amd64   # compiles natively, then assembles the image
```

Both targets build the binary first, so they work from a clean checkout too, and
because the image build has no `RUN` steps an image for another architecture is
assembled without emulation. CI does the same in two steps (`build-binary`, then
`build-docker`) and publishes `registry.wdpkr.home/edh-stats:latest` for
`linux/arm64`, which is what the deploy host pulls; to publish both architectures,
add `linux/amd64` to `platforms` in `.woodpecker/build.yaml`.

## Deployment

`.woodpecker/build.yaml` publishes the image to `registry.wdpkr.home` and then
deploys it over SSH. The deployment directory on the host (`~/edh-stats` by
default) is a **clone of this repository**, and the deploy step checks it out to
the exact revision whose image it just built, before pulling that image:

```sh
cd ~/edh-stats
git fetch --prune origin && git checkout --force "$CI_COMMIT_SHA"
docker compose pull edh-stats && docker compose up -d --force-recreate edh-stats
```

Only `docker-compose.yml` is taken from the repository; `data/` holds the database
and is untracked, so a checkout never touches it. Tracked files must not be edited
on the host - the next deploy overwrites them, so make the change here instead.

## Data directory and permissions

SQLite writes both the database file and its `-wal`/`-shm` siblings, so the
*directory* has to be writable, not just the file.

The image ships `/app/data` pre-owned by its runtime uid (10001), which makes a
plain `docker run edh-stats` and a named volume work with no extra setup:

```sh
docker run -v edh-stats-data:/app/data -p 8080:8080 registry.wdpkr.home/edh-stats:latest
```

`docker-compose.yml` bind-mounts `./data` instead, so that the database file stays
visible on the host. A bind mount hides the image's own directory and keeps the
*host* ownership, so the container is run as that owner instead:

```sh
APP_UID=1000 APP_GID=1000 docker compose up -d   # defaults
```

Two things to check when the service cannot write the database:

* the host directory must be writable by `APP_UID`/`APP_GID` (that is the usual
  fix: `chown -R 1000:1000 data/`);
* under **rootless Docker** or a daemon with `userns-remap`, the container uid is
  mapped to a different host uid, so `APP_UID=1000` no longer matches the host
  owner. Use the named volume above, or `chown` the directory to the mapped uid.


## Migrations

The SQL files under `migrations/` use the [goose](https://github.com/pressly/goose)
naming and annotation format. `migrations/sqlite3` is embedded into the binary and
applied on startup; the `postgres` directory is kept for a future backend and is
not applied.

They can still be applied or inspected by hand with the goose CLI:

```sh
goose -dir migrations/sqlite3 sqlite3 data/edh_stats.db status
goose -dir migrations/sqlite3 sqlite3 data/edh_stats.db up
```

## Secrets and deploy trust

`deploy_key`, `deploy_key.pub` and `deploy_key_nopass` are private keys and are
ignored by git. The key used by the pipeline lives in the Woodpecker secret
`deploy_ssh_key`.

The deploy step builds `known_hosts` with `ssh-keyscan` and keeps host
verification enabled. That still trusts the first answer to the scan; to pin the
host key properly, store the host's public key in a Woodpecker secret
(`deploy_host_key`) and write it into `~/.ssh/known_hosts` in the pipeline instead
of scanning for it.
