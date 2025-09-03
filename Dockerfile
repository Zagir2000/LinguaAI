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

# Финальный образ
FROM ubuntu:20.04

# Build arguments для метаданных
ARG BUILD_DATE
ARG GIT_COMMIT

# Устанавливаем необходимые пакеты включая Festival TTS
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y \
    ca-certificates \
    tzdata \
    festival \
    festvox-kallpc16k \
    festvox-cmu-us-slt-arctic-hts \
    festvox-cmu-us-rms-arctic-hts \
    festvox-cmu-us-awb-arctic-hts \
    && rm -rf /var/lib/apt/lists/*

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

# Создаем директорию для логов
RUN mkdir -p /app/logs && \
    chown -R appuser:appgroup /app

# Переключаемся на непривилегированного пользователя
USER appuser

# Открываем порт (если понадобится для health check)
EXPOSE 8080

# Запускаем приложение
CMD ["./main"] 