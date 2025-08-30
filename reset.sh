#!/bin/bash

# Скрипт для полной очистки Lingua AI приложения

echo "🧹 Полная очистка Lingua AI..."

# Останавливаем контейнеры
docker-compose down

# Удаляем контейнеры и образы
echo "🗑️  Удаление контейнеров и образов..."
docker-compose down --rmi all --volumes --remove-orphans

# Удаляем тома
echo "🗑️  Удаление томов..."
docker volume rm lingua-ai_postgres_data lingua-ai_pgadmin_data 2>/dev/null || true

# Удаляем логи
echo "🗑️  Удаление логов..."
rm -rf logs/* 2>/dev/null || true

# Очищаем Docker cache
echo "🧹 Очистка Docker cache..."
docker system prune -f

echo "✅ Lingua AI полностью очищен!"
echo ""
echo "Для запуска используйте: ./start.sh" 