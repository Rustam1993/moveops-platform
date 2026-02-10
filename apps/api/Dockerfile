FROM golang:1.22-alpine AS builder
WORKDIR /src

RUN apk add --no-cache git
COPY . .

RUN go build -o /out/api ./cmd/server
RUN go build -o /out/migrate ./cmd/migrate
RUN go build -o /out/seed ./cmd/seed

FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/api /usr/local/bin/api
COPY --from=builder /out/migrate /usr/local/bin/migrate
COPY --from=builder /out/seed /usr/local/bin/seed
COPY --from=builder /src/openapi.yaml /app/openapi.yaml
COPY --from=builder /src/migrations /app/migrations
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

RUN chmod +x /usr/local/bin/docker-entrypoint.sh

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
