# 🚀 Lingua AI - Telegram Bot для изучения языков

**Lingua AI** - это интеллектуальный Telegram бот для изучения английского языка с использованием AI-технологий. Бот предоставляет персонализированное обучение, адаптируясь к уровню пользователя.

## ✨ **Основные возможности**

### 🎯 **Система уровней и тестирования**
- **Автоматическое определение уровня** пользователя
- **Адаптивные тесты** для оценки знаний
- **Динамическое изменение уровня** на основе результатов

### 🎭 **Диалоги-сценарии**
- **Интерактивные диалоги** на английском языке
- **Реалистичные сценарии:** ресторан, магазин, путешествия
- **Адаптивная сложность** под уровень пользователя
- **AI-генерация** персонализированного контента
- **Отслеживание прогресса** и статистика

### 🗣️ **Голосовые сообщения**
- **Распознавание речи** с помощью Whisper AI
- **Поддержка смешанных языков** (русский + английский)
- **Voice Activity Detection (VAD)** для улучшения качества
- **Автоматическая транскрипция** и перевод

### 🎓 **Карточки для запоминания (Flashcards)**
- **Персонализированные карточки** на основе ошибок
- **Система повторений** с интервалами
- **Отслеживание прогресса** изучения слов
- **Адаптивная сложность** карточек


### 📊 **Статистика и аналитика**
- **Детальная статистика** обучения
- **Отслеживание прогресса** по уровням
- **Анализ ошибок** и рекомендации
- **История активности** пользователя

## 🏗️ **Архитектура системы**

### **Основные компоненты:**
- **Telegram Bot Handler** - обработка сообщений и команд
- **AI Service** - интеграция с GigaChat API
- **Dialogue Service** - управление диалогами-сценариями
- **Whisper Service** - распознавание речи
- **User Service** - управление пользователями и уровнями
- **PostgreSQL** - хранение данных и прогресса

### **Технологический стек:**
- **Backend:** Go 1.23
- **База данных:** PostgreSQL 15
- **AI:** GigaChat API, Whisper AI
- **Контейнеризация:** Docker + Docker Compose
- **Логирование:** Zap logger
- **Метрики:** Prometheus-совместимые

## 🚀 **Быстрый старт**

### **1. Клонирование репозитория**
```bash
git clone <repository-url>
cd "Lingua AI"
```

### **2. Настройка окружения**
```bash
cp .env.example .env
# Отредактируйте .env файл с вашими настройками
```

### **3. Запуск сервисов**
```bash
# Запуск всех сервисов
./scripts/start.sh

# Или через Docker Compose
docker-compose up -d
```

### **4. Проверка статуса**
```bash
# Статус контейнеров
docker-compose ps

# Логи приложения
docker-compose logs -f app
```

## ⚙️ **Конфигурация**

### **Основные переменные окружения:**
```bash
# Telegram Bot Configuration
TELEGRAM_BOT_TOKEN=your_telegram_bot_token_here
TELEGRAM_WEBHOOK_URL=https://lingua-ai.ru/webhook/telegram

# AI Provider Configuration
AI_PROVIDER=deepseek  # deepseek или openrouter
AI_MODEL=deepseek-chat
AI_MAX_TOKENS=1000
AI_TEMPERATURE=0.7

# DeepSeek Configuration (основной провайдер)
DEEPSEEK_API_KEY=your_deepseek_api_key_here
DEEPSEEK_BASE_URL=https://api.deepseek.com/v1

# Whisper Configuration
WHISPER_API_URL=http://whisper:9000
WHISPER_MODEL=small  # tiny, base, small, medium, large
WHISPER_COMPUTE=int8  # int8 (быстро) или float32 (качество)

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=lingua_user
DB_PASSWORD=lingua_password
DB_NAME=lingua_ai
DB_SSL_MODE=disable

# Application Configuration
APP_ENV=development
LOG_LEVEL=debug
APP_PORT=8080

# WebApp Configuration
WEBAPP_URL=https://your-domain.com

# YooKassa Configuration
YUKASSA_SHOP_ID=test_shop_id
YUKASSA_SECRET_KEY=test_secret_key
YUKASSA_TEST_MODE=true

# Migration Configuration
MIGRATION_PATH=file://scripts/migrations
```

## 📁 **Структура проекта**

```
Lingua AI/
├── cmd/                    # Точка входа приложения
│   └── main.go
├── internal/               # Внутренняя логика
│   ├── ai/                # AI клиенты и интеграции
│   ├── audio/             # Обработка аудио (VAD)
│   ├── bot/               # Telegram bot логика
│   ├── config/            # Конфигурация приложения
│   ├── dialogue/          # Система диалогов-сценариев
│   ├── metrics/           # Метрики и мониторинг
│   ├── store/             # Работа с базой данных
│   ├── user/              # Управление пользователями
│   └── whisper/           # Whisper API клиент
├── pkg/                   # Публичные пакеты
│   └── models/            # Модели данных
├── scripts/               # Скрипты и миграции
│   ├── migrations/        # SQL миграции
│   ├── start.sh          # Запуск сервисов
│   ├── stop.sh           # Остановка сервисов
│   ├── restart.sh        # Перезапуск сервисов
│   └── reset.sh          # Полный сброс
├── docker-compose.yml     # Docker конфигурация
├── Dockerfile             # Docker образ приложения
└── README.md              # Документация
```

## 🎮 **Использование бота**

### **Основные команды:**
- `/start` - начало работы с ботом
- `/help` - справка по командам
- `/level` - пройти тест на определение уровня
- `/flashcards` - начать изучение карточек
- `/stats` - ваша статистика обучения

### **Интерактивные функции:**
- **Голосовые сообщения** - отправьте аудио для транскрипции
- **Текстовые сообщения** - получите перевод и объяснение
- **Карточки** - изучайте слова с интервальными повторениями

## 🔧 **Управление сервисами**

### **Скрипты управления:**
```bash
# Запуск
./scripts/start.sh

# Остановка
./scripts/stop.sh

# Перезапуск
./scripts/restart.sh

# Полный сброс (ОСТОРОЖНО!)
./scripts/reset.sh
```

### **Docker команды:**
```bash
# Статус сервисов
docker-compose ps

# Логи конкретного сервиса
docker-compose logs -f app
docker-compose logs -f postgres
docker-compose logs -f whisper

# Перезапуск без потери данных
docker-compose restart

# Пересборка и запуск
docker-compose up -d --build
```

## 📊 **Мониторинг и логи**

### **Просмотр логов:**
```bash
# Все сервисы
docker-compose logs -f

# Конкретный сервис
docker-compose logs -f app

# Последние 100 строк
docker-compose logs --tail=100 app
```

### **Метрики:**
- **AI запросы** - количество и время ответа
- **Пользователи** - активность и прогресс
- **Диалоги** - завершенные сценарии
- **Карточки** - изученные слова

## 🗄️ **База данных**

### **Основные таблицы:**
- `users` - пользователи и их уровни
- `dialogue_scenarios` - доступные сценарии диалогов
- `dialogue_sessions` - активные сессии пользователей
- `dialogue_choices` - выборы пользователей в диалогах
- `dialogue_stats` - статистика прохождения диалогов
- `flashcard_sessions` - сессии изучения карточек

### **Миграции:**
```bash
# Применение миграций
docker-compose exec postgres goose -dir /migrations up

# Откат миграций
docker-compose exec postgres goose -dir /migrations down
```

---

**Lingua AI** - делаем изучение языков умным и увлекательным! 🎯✨ 
