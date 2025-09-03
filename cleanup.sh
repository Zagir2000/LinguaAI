#!/bin/bash

# Скрипт для очистки старых сообщений пользователей
# Использование: ./cleanup.sh [опции]

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Функция для вывода справки
show_help() {
    echo -e "${BLUE}Утилита очистки старых сообщений Lingua AI${NC}"
    echo ""
    echo "Использование: $0 [ОПЦИИ]"
    echo ""
    echo "ОПЦИИ:"
    echo "  -k, --keep COUNT     Количество сообщений для сохранения (по умолчанию: 10)"
    echo "  -u, --user ID        ID пользователя для очистки (0 = все пользователи)"
    echo "  -d, --dry-run        Показать что будет удалено без фактического удаления"
    echo "  -h, --help           Показать эту справку"
    echo ""
    echo "ПРИМЕРЫ:"
    echo "  $0                           # Очистить всех пользователей, оставив по 10 сообщений"
    echo "  $0 -k 20                     # Оставить по 20 сообщений для каждого пользователя"
    echo "  $0 -u 12345 -k 7             # Очистить пользователя 12345, оставив 7 сообщений"
    echo "  $0 -d                        # Сухой прогон - показать что будет удалено"
    echo ""
}

# Значения по умолчанию
KEEP_COUNT=10
USER_ID=0
DRY_RUN=false

# Парсинг аргументов
while [[ $# -gt 0 ]]; do
    case $1 in
        -k|--keep)
            KEEP_COUNT="$2"
            shift 2
            ;;
        -u|--user)
            USER_ID="$2"
            shift 2
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo -e "${RED}Неизвестная опция: $1${NC}"
            show_help
            exit 1
            ;;
    esac
done

# Проверка валидности аргументов
if ! [[ "$KEEP_COUNT" =~ ^[0-9]+$ ]] || [ "$KEEP_COUNT" -lt 1 ]; then
    echo -e "${RED}Ошибка: Количество сообщений должно быть положительным числом${NC}"
    exit 1
fi

if ! [[ "$USER_ID" =~ ^[0-9]+$ ]]; then
    echo -e "${RED}Ошибка: ID пользователя должен быть числом${NC}"
    exit 1
fi

# Проверка существования бинарного файла
CLEANUP_BIN="./cmd/cleanup/cleanup"
if [ ! -f "$CLEANUP_BIN" ]; then
    echo -e "${YELLOW}Бинарный файл не найден. Компилируем...${NC}"
    
    # Компиляция утилиты очистки
    cd cmd/cleanup
    go build -o cleanup main.go
    cd ../..
    
    if [ ! -f "$CLEANUP_BIN" ]; then
        echo -e "${RED}Ошибка компиляции утилиты очистки${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}Утилита успешно скомпилирована${NC}"
fi

# Формирование команды
CMD_ARGS="-keep $KEEP_COUNT"

if [ "$USER_ID" -gt 0 ]; then
    CMD_ARGS="$CMD_ARGS -user $USER_ID"
fi

if [ "$DRY_RUN" = true ]; then
    CMD_ARGS="$CMD_ARGS -dry-run"
fi

# Вывод информации о запуске
echo -e "${BLUE}=== Очистка сообщений Lingua AI ===${NC}"
echo -e "Количество сообщений для сохранения: ${GREEN}$KEEP_COUNT${NC}"

if [ "$USER_ID" -gt 0 ]; then
    echo -e "Пользователь: ${GREEN}$USER_ID${NC}"
else
    echo -e "Пользователи: ${GREEN}Все${NC}"
fi

if [ "$DRY_RUN" = true ]; then
    echo -e "Режим: ${YELLOW}Сухой прогон (без фактического удаления)${NC}"
else
    echo -e "Режим: ${RED}Реальное удаление${NC}"
fi

echo ""

# Подтверждение для реального удаления
if [ "$DRY_RUN" = false ]; then
    echo -e "${YELLOW}ВНИМАНИЕ: Это действие необратимо!${NC}"
    read -p "Продолжить? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${BLUE}Операция отменена${NC}"
        exit 0
    fi
fi

# Запуск утилиты очистки
echo -e "${BLUE}Запуск очистки...${NC}"
echo ""

if $CLEANUP_BIN $CMD_ARGS; then
    echo ""
    echo -e "${GREEN}✓ Очистка завершена успешно${NC}"
else
    echo ""
    echo -e "${RED}✗ Ошибка при выполнении очистки${NC}"
    exit 1
fi 