# syntax=docker/dockerfile:1

FROM golang:1.26.4-alpine AS build

WORKDIR /src

# Dependencies change far less often than sources, so resolve them in their own
# layer. No BuildKit cache mounts here: Cloud Build's --source path still uses
# the legacy Docker builder, which does not understand them.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/marko .

# The binary is static and talks to nobody, so it needs no OS underneath it.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/marko /marko

ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/marko"]
