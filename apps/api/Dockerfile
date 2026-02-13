FROM golang:1.24.13-alpine AS builder
WORKDIR /src/apps/api

RUN apk add --no-cache git

# Copy module files first for better layer caching.
COPY apps/api/go.mod apps/api/go.sum ./
RUN go mod download

# Copy API source.
COPY apps/api ./

RUN go build -o /out/api ./cmd/server
RUN go build -o /out/migrate ./cmd/migrate
RUN go build -o /out/seed ./cmd/seed

FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/api /usr/local/bin/api
COPY --from=builder /out/migrate /usr/local/bin/migrate
COPY --from=builder /out/seed /usr/local/bin/seed
COPY --from=builder /src/apps/api/openapi.yaml /app/openapi.yaml
COPY --from=builder /src/apps/api/migrations /app/migrations
COPY apps/api/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

RUN chmod +x /usr/local/bin/docker-entrypoint.sh

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
