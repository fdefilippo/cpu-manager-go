FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .

RUN apk add --no-cache git make \
    && go mod download \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o cpu-manager-go

FROM alpine:3.18

RUN apk add --no-cache ca-certificates tzdata

# Crea utente non-root per sicurezza
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /app/cpu-manager-go /usr/local/bin/cpu-manager-go
COPY --from=builder /app/config/cpu-manager.conf.example /etc/cpu-manager.conf

# Permetti all'utente di scrivere nei log
RUN mkdir -p /var/log /var/run/cpu-manager \
    && chown -R appuser:appgroup /var/log /var/run/cpu-manager \
    && chmod 755 /usr/local/bin/cpu-manager-go

USER appuser
WORKDIR /home/appuser

# Esponi porta Prometheus
EXPOSE 9101

# Volume per configurazione esterna
VOLUME ["/etc/cpu-manager.conf"]

ENTRYPOINT ["cpu-manager-go"]
CMD ["--config", "/etc/cpu-manager.conf"]
