#!/bin/bash

# Скрипт для сборки базового образа с Mozilla TTS

set -e

echo "🚀 Сборка базового образа с Mozilla TTS..."

# Проверяем, есть ли Docker
if ! command -v docker &> /dev/null; then
    echo "❌ Docker не установлен"
    exit 1
fi

# Собираем базовый образ
echo "🔨 Собираем базовый образ..."

docker build \
    -f Dockerfile.base \
    --tag lingua-ai-base:latest \
    --tag ghcr.io/zagir2000/linguaai-base:latest \
    .

echo "✅ Базовый образ успешно собран!"
echo "🏷️  Теги:"
echo "   - lingua-ai-base:latest"
echo "   - ghcr.io/zagir2000/linguaai-base:latest"

# Показываем размер образа
echo "📊 Размер базового образа:"
docker images lingua-ai-base:latest --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"

echo ""
echo "💡 Теперь можно собирать основной образ быстрее:"
echo "   docker build -t lingua-ai:latest ."
