FROM golang:1.24-alpine AS builder

WORKDIR /app

# Устанавливаем зависимости для сборки
#RUN #apk add --no-cache git ca-certificates
RUN set -eux; \
    printf '%s\n' \
      'https://dl-cdn.alpinelinux.org/alpine/v3.22/main' \
      'https://dl-cdn.alpinelinux.org/alpine/v3.22/community' > /etc/apk/repositories; \
    ok=0; \
    for i in 1 2 3; do \
      if apk add --no-cache git ca-certificates gcc musl-dev; then ok=1; break; fi; \
      echo "apk install failed, retry $i/3"; \
      sleep $((i * 3)); \
    done; \
    [ "$ok" -eq 1 ]

# Копируем go.mod и go.sum
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь проект
COPY . .

# Собираем приложение
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o /go/bin/bot ./cmd/bot
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o /go/bin/api ./cmd/api

# Финальный образ
FROM alpine:latest

#RUN #apk --no-cache add ca-certificates tzdata
RUN set -eux; \
    printf '%s\n' \
      'https://dl-cdn.alpinelinux.org/alpine/v3.22/main' \
      'https://dl-cdn.alpinelinux.org/alpine/v3.22/community' > /etc/apk/repositories; \
    ok=0; \
    for i in 1 2 3; do \
      if apk add --no-cache ca-certificates tzdata sqlite-libs; then ok=1; break; fi; \
      echo "apk install failed, retry $i/3"; \
      sleep $((i * 3)); \
    done; \
    [ "$ok" -eq 1 ]

WORKDIR /app/

# Копируем бинарники из builder
COPY --from=builder /go/bin/bot .
COPY --from=builder /go/bin/api .

# Создаем папку для конфигов
RUN mkdir /configs

# Добавляем HEALTHCHECK
HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --spider http://localhost:8080/healthz || exit 1

# Указываем команду запуска
CMD ["./bot"]
