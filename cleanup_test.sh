#!/bin/bash

echo "🧹 Очистка тестовых файлов..."

# Удаляем тестовые файлы
rm -f test_payments
rm -f test_payments.go

echo "✅ Тестовые файлы удалены!"

# Оставляем только документацию
echo "📚 Документация сохранена:"
echo "   • PAYMENT_TESTING.md - инструкции по тестированию"
echo "   • env.example - пример конфигурации"
