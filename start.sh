#!/bin/bash

# Скрипт для запуска Lingua AI приложения

set -e

echo "🚀 Запуск Lingua AI..."

# Проверяем наличие .env файла
if [ ! -f .env ]; then
    echo "❌ Файл .env не найден!"
    echo "📝 Создайте файл .env на основе env.example"
    echo "cp env.example .env"
    exit 1
fi

# Проверяем обязательные переменные окружения
source .env

if [ -z "$TELEGRAM_BOT_TOKEN" ]; then
    echo "❌ TELEGRAM_BOT_TOKEN не установлен в .env файле"
    exit 1
fi

if [ -z "$DEEPSEEK_API_KEY" ]; then
    echo "❌ DEEPSEEK_API_KEY не установлен в .env файле"
    exit 1
fi

echo "✅ Переменные окружения проверены"

# Создаем директорию для логов
mkdir -p logs

# Запускаем контейнеры
echo "🐳 Запуск Docker контейнеров..."
docker-compose up -d

# Ждем запуска базы данных
echo "⏳ Ожидание запуска базы данных..."
sleep 10

# Проверяем статус контейнеров
echo "📊 Статус контейнеров:"
docker-compose ps

echo "✅ Lingua AI запущен!"
echo ""
echo "📱 Telegram бот готов к работе"
echo "🗄️  База данных: localhost:5432"
echo "📊 pgAdmin (опционально): localhost:5050"
echo "📝 Логи: ./logs/"
echo ""
echo "Для просмотра логов используйте: ./logs.sh"
echo "Для остановки используйте: ./stop.sh" 