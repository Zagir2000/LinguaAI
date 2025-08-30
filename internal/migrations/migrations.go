package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"lingua-ai/internal/config"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
)

// RunMigrations применяет миграции к базе данных
func RunMigrations(cfg *config.Config, logger *zap.Logger) error {
	logger.Info("начало применения миграций")

	// Устанавливаем путь к миграциям
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("ошибка установки диалекта: %w", err)
	}

	// Создаем временное подключение к базе данных для миграций
	dsn := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("ошибка подключения к базе данных для миграций: %w", err)
	}
	defer db.Close()

	// Определяем правильный путь к миграциям
	migrationPath := getMigrationPath(cfg.Database.MigrationPath, logger)

	// Применяем миграции
	if err := goose.Up(db, migrationPath); err != nil {
		return fmt.Errorf("ошибка применения миграций: %w", err)
	}

	logger.Info("миграции успешно применены")
	return nil
}

// GetMigrationStatus возвращает статус миграций
func GetMigrationStatus(cfg *config.Config, logger *zap.Logger) error {
	logger.Info("проверка статуса миграций")

	// Устанавливаем путь к миграциям
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("ошибка установки диалекта: %w", err)
	}

	// Создаем временное подключение к базе данных для миграций
	dsn := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("ошибка подключения к базе данных для миграций: %w", err)
	}
	defer db.Close()

	// Определяем правильный путь к миграциям
	migrationPath := getMigrationPath(cfg.Database.MigrationPath, logger)

	// Получаем статус миграций
	if err := goose.Status(db, migrationPath); err != nil {
		return fmt.Errorf("ошибка получения статуса миграций: %w", err)
	}

	logger.Info("статус миграций получен")
	return nil
}

// getMigrationPath определяет правильный путь к миграциям
func getMigrationPath(configPath string, logger *zap.Logger) string {
	// Сначала проверяем, существует ли путь из конфигурации
	if _, err := os.Stat(configPath); err == nil {
		logger.Info("используем путь к миграциям из конфигурации", zap.String("path", configPath))
		return configPath
	}

	// Если не существует, пробуем найти в текущей директории
	currentDir, err := os.Getwd()
	if err != nil {
		logger.Warn("не удалось получить текущую директорию, используем путь из конфигурации", zap.Error(err))
		return configPath
	}

	// Пробуем разные варианты путей
	possiblePaths := []string{
		filepath.Join(currentDir, "scripts", "migrations"),
		filepath.Join(currentDir, "migrations"),
		filepath.Join(currentDir, "..", "scripts", "migrations"),
		filepath.Join(currentDir, "..", "migrations"),
		"/app/scripts/migrations", // Для Docker контейнера
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			logger.Info("найден путь к миграциям", zap.String("path", path))
			return path
		}
	}

	// Если ничего не найдено, возвращаем исходный путь
	logger.Warn("не удалось найти директорию с миграциями, используем путь из конфигурации", zap.String("path", configPath))
	return configPath
}
