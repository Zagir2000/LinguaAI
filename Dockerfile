# Используем официальный образ Go (последняя версия)
FROM golang:1.23-alpine AS builder

# Build arguments для принудительного обновления
ARG BUILD_DATE
ARG GIT_COMMIT

# Устанавливаем необходимые пакеты
RUN apk add --no-cache git ca-certificates tzdata

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем файлы зависимостей
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd

# Используем предварительно собранный базовый образ с TTS
FROM ghcr.io/zagir2000/linguaai-base:latest AS final

# Добавляем метаданные
LABEL build_date="$BUILD_DATE" \
      git_commit="$GIT_COMMIT" \
      maintainer="Lingua AI Team"

# Создаем пользователя для безопасности
RUN groupadd -g 1001 appgroup && \
    useradd -u 1001 -g appgroup -s /bin/bash -m appuser

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем бинарный файл из builder
COPY --from=builder /app/main .

# Копируем миграции
COPY --from=builder /app/scripts ./scripts

# Создаем директорию для логов и очищаем кэш
RUN mkdir -p /app/logs && \
    chown -R appuser:appgroup /app && \
    rm -rf /tmp/* /var/tmp/* /root/.cache

# Переключаемся на непривилегированного пользователя
USER appuser

# Открываем порт (если понадобится для health check)
EXPOSE 8080

# Запускаем приложение
CMD ["./main"] 