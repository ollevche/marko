# marko

A plain HTTP server (not a cloud function) that receives Bluedot meeting
transcript webhooks and prints them to stdout.

It serves exactly one endpoint:

```
POST /meetings/transcripts
```

Bluedot delivers webhooks through [Svix](https://docs.svix.com/receiving/verifying-payloads/how),
so every request is verified against the signing secret before anything is
printed. Unsigned or badly signed requests get a `401`.

## Running

```sh
export BLUEDOT_WEBHOOK_SECRET=whsec_...   # from Bluedot's webhook settings
go run .
```

| Variable | Default | Meaning |
| --- | --- | --- |
| `BLUEDOT_WEBHOOK_SECRET` | *(required)* | Svix signing secret; the server refuses to start without it. |
| `PORT` | `8080` | Port to listen on. |

`SIGINT`/`SIGTERM` triggers a graceful shutdown.

## Sending a test webhook

Signed requests are awkward to build by hand, so there is a helper:

```sh
export BLUEDOT_WEBHOOK_SECRET=whsec_$(openssl rand -base64 24)
go run . &
./scripts/send-test-webhook.sh              # built-in sample payload
./scripts/send-test-webhook.sh payload.json # or your own
```

The server prints the verified payload and answers `ok`.

## Docker

```sh
docker build -t marko .
docker run --rm -p 8080:8080 -e BLUEDOT_WEBHOOK_SECRET=whsec_... marko
```

Multi-stage build: compiled with `golang:1.26.4-alpine`, shipped on
`distroless/static` as a non-root user with nothing but the static binary.

## Layout

- `main.go` — configuration, routing, server lifecycle.
- `transcripts.go` — webhook verification and printing.
- `scripts/send-test-webhook.sh` — signs and posts a test payload locally.
- `Dockerfile` — multi-stage build to a distroless image.
