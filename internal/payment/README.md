# Telegram Payments API для Lingua AI

Этот пакет реализует полную интеграцию с Telegram Payments API для приема платежей через YooKassa.

## 🚀 Возможности

- ✅ **sendInvoice** - отправка счетов на оплату
- ✅ **Shipping Query** - обработка запросов о доставке
- ✅ **PreCheckout Query** - предварительная проверка платежей
- ✅ **Successful Payment** - обработка успешных платежей
- ✅ **Автоматическая активация премиум подписки**

## 📋 Требования

### 1. Настройка бота в @BotFather
```bash
# Подключение к YooKassa
/payments
# Выберите YooKassa
# Получите provider_token
```

### 2. Переменные окружения
```bash
TELEGRAM_BOT_TOKEN=your_bot_token
YUKASSA_PROVIDER_TOKEN=your_yookassa_provider_token
```

## 🔧 Использование

### Инициализация сервиса
```go
import "lingua-ai/internal/payment"

// Создаем сервис
telegramService := payment.NewTelegramPaymentService(
    botToken, 
    providerToken,
)
```

### Отправка счета на премиум подписку
```go
// Создаем счет для месячной подписки
invoice := telegramService.CreatePremiumInvoice(
    userID,           // ID пользователя Telegram
    "month",          // План подписки
    30,               // Длительность в днях
    29900,            // Стоимость в копейках (299 рублей)
)

// Отправляем счет
err := telegramService.SendInvoice(chatID, invoice)
if err != nil {
    log.Printf("Ошибка отправки счета: %v", err)
}
```

### Обработка webhook'ов
```go
// Создаем обработчик
webhookHandler := payment.NewWebhookHandler(
    telegramService,
    userService,      // Ваш сервис пользователей
    paymentService,   // Ваш сервис платежей
)

// Обрабатываем webhook
err := webhookHandler.HandleWebhook(webhookData)
```

## 📊 Структура payload

Для премиум подписки используется формат:
```
premium_USERID_PLAN_DURATION
```

**Примеры:**
- `premium_123456789_month_30` - месячная подписка
- `premium_123456789_year_365` - годовая подписка
- `premium_123456789_custom_7` - недельная подписка

## 🎯 Поддерживаемые планы

| План | Длительность | Описание |
|------|--------------|----------|
| `month` | 30 дней | Месячная подписка |
| `quarter` | 90 дней | Квартальная подписка |
| `year` | 365 дней | Годовая подписка |

## 🔄 Жизненный цикл платежа

1. **Отправка счета** → `SendInvoice()`
2. **Запрос доставки** → `handleShippingQuery()`
3. **Предварительная проверка** → `handlePreCheckoutQuery()`
4. **Успешный платеж** → `handleSuccessfulPayment()`
5. **Активация премиума** → Автоматически

## 🛡️ Безопасность

- ✅ Валидация payload
- ✅ Проверка существования пользователя
- ✅ Валидация планов подписки
- ✅ Логирование всех операций
- ✅ Обработка ошибок

## 📝 Логирование

Все операции логируются:
```
2025/08/30 18:30:00 Получен запрос о доставке от пользователя 123456789
2025/08/30 18:30:05 Получена предварительная проверка платежа от пользователя 123456789
2025/08/30 18:30:10 Получен успешный платеж: yookassa_transaction_id
2025/08/30 18:30:10 Премиум подписка активирована для пользователя 123456789 на 30 дней
```

## 🚨 Обработка ошибок

- **Неверный payload** → Отклонение платежа
- **Пользователь не найден** → Отклонение платежа
- **Неверный план** → Отклонение платежа
- **Ошибки API** → Логирование и повторные попытки

## 🔗 Интеграция с существующим кодом

### 1. Добавьте в конфигурацию
```go
type Config struct {
    // ... существующие поля
    TelegramBotToken    string
    YooKassaProviderToken string
}
```

### 2. Инициализируйте в main.go
```go
telegramService := payment.NewTelegramPaymentService(
    cfg.TelegramBotToken,
    cfg.YooKassaProviderToken,
)
```

### 3. Добавьте в webhook handler
```go
webhookHandler := payment.NewWebhookHandler(
    telegramService,
    userService,
    paymentService,
)
```

## 📚 Дополнительные ресурсы

- [Telegram Bot API - sendInvoice](https://core.telegram.org/bots/api#sendinvoice)
- [Telegram Payments](https://core.telegram.org/bots/payments)
- [YooKassa API](https://yookassa.ru/developers/api)

## 🆘 Поддержка

При возникновении проблем:
1. Проверьте логи приложения
2. Убедитесь в правильности токенов
3. Проверьте настройки webhook'а
4. Убедитесь в доступности YooKassa API
