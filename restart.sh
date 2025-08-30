#!/bin/bash

# Скрипт для перезапуска Lingua AI приложения

echo "🔄 Перезапуск Lingua AI..."

# Останавливаем контейнеры
docker-compose down

# Ждем немного
sleep 2

# Запускаем контейнеры
docker-compose up -d

# Ждем запуска
echo "⏳ Ожидание запуска..."
sleep 10

# Проверяем статус
echo "📊 Статус контейнеров:"
docker-compose ps

echo "✅ Lingua AI перезапущен!" 