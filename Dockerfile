# syntax=docker/dockerfile:1

FROM golang:1.26.4-alpine AS go-build

WORKDIR /src

# Dependencies change far less often than sources, so resolve them in their own
# layer. No BuildKit cache mounts here: Cloud Build's --source path still uses
# the legacy Docker builder, which does not understand them.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -C cmd/ -trimpath -ldflags='-s -w' -o /out/marko .

# obsidian-headless pulls in better-sqlite3, a native module. Install it in the
# full Node image, which carries a compiler in case no prebuilt binary matches
# this platform, and hand the result to the slim runtime below.
FROM node:22-bookworm AS node-build

RUN npm install -g obsidian-headless@0.0.14

# Node 22 is the floor obsidian-headless declares, and its native module was
# compiled against this exact runtime one stage up.
FROM node:22-bookworm-slim

# The slim image apt-get purges its build-time packages, ca-certificates among
# them. Node ships its own CA bundle so the Obsidian CLI never noticed, but the
# static Go binary reads the system trust store and cannot verify api.github.com
# without it.
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Copying the whole directory rather than just the package keeps whatever npm
# decided to hoist out of the package's own node_modules. Both images are the
# same Node build, so the npm and corepack copies coming along are identical.
COPY --from=node-build /usr/local/lib/node_modules /usr/local/lib/node_modules
RUN chmod +x /usr/local/lib/node_modules/obsidian-headless/cli.js && \
    ln -s ../lib/node_modules/obsidian-headless/cli.js /usr/local/bin/ob

COPY --from=go-build /out/marko /usr/local/bin/marko

ENV PORT=8080
EXPOSE 8080

# Runs as node, so the vault has to live somewhere node owns. This also gives
# relative paths a writable base: the Obsidian CLI resolves --path against the
# working directory, and / is root-owned.
WORKDIR /home/node
USER node

ENTRYPOINT ["/usr/local/bin/marko"]
