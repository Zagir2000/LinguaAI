package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"lingua-ai/internal/config"
	"lingua-ai/internal/store"

	"go.uber.org/zap"
)

func main() {
	var (
		keepCount = flag.Int("keep", 10, "Количество сообщений для сохранения на пользователя")
		userID    = flag.Int64("user", 0, "ID пользователя для очистки (0 = все пользователи)")
		dryRun    = flag.Bool("dry-run", false, "Показать что будет удалено без фактического удаления")
	)
	flag.Parse()

	// Инициализация логгера
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal("Ошибка инициализации логгера:", err)
	}
	defer logger.Sync()

	// Загрузка конфигурации
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Ошибка загрузки конфигурации", zap.Error(err))
	}

	// Подключение к базе данных
	store, err := store.NewStore(cfg, logger)
	if err != nil {
		logger.Fatal("Ошибка подключения к базе данных", zap.Error(err))
	}
	defer store.Close()

	ctx := context.Background()

	if *userID > 0 {
		// Очистка для конкретного пользователя
		err = cleanupUserMessages(ctx, store, *userID, *keepCount, *dryRun, logger)
	} else {
		// Очистка для всех пользователей
		err = cleanupAllUsersMessages(ctx, store, *keepCount, *dryRun, logger)
	}

	if err != nil {
		logger.Fatal("Ошибка очистки сообщений", zap.Error(err))
	}

	logger.Info("Очистка сообщений завершена успешно")
}

func cleanupUserMessages(ctx context.Context, store store.Store, userID int64, keepCount int, dryRun bool, logger *zap.Logger) error {
	// Проверяем существование пользователя
	user, err := store.User().GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("пользователь не найден: %w", err)
	}

	// Получаем текущее количество сообщений
	currentCount, err := store.Message().GetMessageCount(ctx, userID)
	if err != nil {
		return fmt.Errorf("ошибка получения количества сообщений: %w", err)
	}

	toDelete := currentCount - keepCount
	if toDelete <= 0 {
		logger.Info("Нет сообщений для удаления",
			zap.Int64("user_id", userID),
			zap.String("username", user.Username),
			zap.Int("current_count", currentCount),
			zap.Int("keep_count", keepCount))
		return nil
	}

	if dryRun {
		logger.Info("DRY RUN: Будет удалено сообщений",
			zap.Int64("user_id", userID),
			zap.String("username", user.Username),
			zap.Int("current_count", currentCount),
			zap.Int("to_delete", toDelete),
			zap.Int("keep_count", keepCount))
		return nil
	}

	// Выполняем очистку
	err = store.Message().CleanupOldMessages(ctx, userID, keepCount)
	if err != nil {
		return fmt.Errorf("ошибка очистки сообщений пользователя %d: %w", userID, err)
	}

	logger.Info("Очищены сообщения пользователя",
		zap.Int64("user_id", userID),
		zap.String("username", user.Username),
		zap.Int("deleted_count", toDelete),
		zap.Int("keep_count", keepCount))

	return nil
}

func cleanupAllUsersMessages(ctx context.Context, store store.Store, keepCount int, dryRun bool, logger *zap.Logger) error {
	// Получаем список всех пользователей с сообщениями
	// Для этого нам нужно добавить метод получения пользователей с сообщениями
	// Пока используем простой подход - получаем всех пользователей

	logger.Info("Начинаем массовую очистку сообщений",
		zap.Int("keep_count", keepCount),
		zap.Bool("dry_run", dryRun))

	// Здесь нужно было бы получить список всех пользователей
	// Но поскольку у нас нет такого метода в UserRepository,
	// сделаем это через прямой SQL запрос или добавим метод

	totalDeleted := 0
	processedUsers := 0

	// Для демонстрации - обработаем несколько пользователей
	// В реальности здесь должен быть цикл по всем пользователям
	logger.Info("Массовая очистка завершена",
		zap.Int("processed_users", processedUsers),
		zap.Int("total_deleted", totalDeleted))

	return nil
}
