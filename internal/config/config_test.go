package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// cleanupEnv очищает переменные окружения после теста
func cleanupEnv() {
	os.Unsetenv("TELEGRAM_BOT_TOKEN")
	os.Unsetenv("AI_PROVIDER")
	os.Unsetenv("OPENROUTER_API_KEY")
	os.Unsetenv("DEEPSEEK_API_KEY")
	os.Unsetenv("YUKASSA_PROVIDER_TOKEN")
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_NAME")
}

func TestLoadConfig(t *testing.T) {
	// Устанавливаем переменные окружения для теста
	os.Setenv("TELEGRAM_BOT_TOKEN", "test_token")
	os.Setenv("AI_PROVIDER", "openrouter")
	os.Setenv("OPENROUTER_API_KEY", "test_openrouter_key")
	os.Setenv("YUKASSA_PROVIDER_TOKEN", "test_provider_token")
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_USER", "test_user")
	os.Setenv("DB_PASSWORD", "test_password")
	os.Setenv("DB_NAME", "test_db")

	// Очищаем переменные после теста
	defer cleanupEnv()

	// Загружаем конфигурацию
	cfg, err := Load()

	// Проверяем, что конфигурация загружена без ошибок
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Проверяем значения
	assert.Equal(t, "test_token", cfg.Telegram.BotToken)
	assert.Equal(t, "openrouter", cfg.AI.Provider)
	assert.Equal(t, "test_openrouter_key", cfg.AI.OpenRouter.APIKey)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, "test_user", cfg.Database.User)
	assert.Equal(t, "test_password", cfg.Database.Password)
	assert.Equal(t, "test_db", cfg.Database.Name)

	// Проверяем значения по умолчанию
	assert.Equal(t, "deepseek-chat", cfg.AI.Model)
	assert.Equal(t, 1000, cfg.AI.MaxTokens)
	assert.Equal(t, 0.7, cfg.AI.Temperature)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "disable", cfg.Database.SSLMode)
	assert.Equal(t, "development", cfg.App.Env)
	assert.Equal(t, "info", cfg.App.LogLevel)
	assert.Equal(t, 8080, cfg.App.Port)
}

func TestLoadConfigDeepSeek(t *testing.T) {
	// Устанавливаем переменные окружения для теста DeepSeek
	os.Setenv("TELEGRAM_BOT_TOKEN", "test_token")
	os.Setenv("AI_PROVIDER", "deepseek")
	os.Setenv("DEEPSEEK_API_KEY", "test_deepseek_key")
	os.Setenv("YUKASSA_PROVIDER_TOKEN", "test_provider_token")
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_USER", "test_user")
	os.Setenv("DB_PASSWORD", "test_password")
	os.Setenv("DB_NAME", "test_db")

	// Очищаем переменные после теста
	defer cleanupEnv()

	// Загружаем конфигурацию
	cfg, err := Load()

	// Проверяем, что конфигурация загружена без ошибок
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// Проверяем значения
	assert.Equal(t, "test_token", cfg.Telegram.BotToken)
	assert.Equal(t, "deepseek", cfg.AI.Provider)
	assert.Equal(t, "test_deepseek_key", cfg.AI.DeepSeek.APIKey)
	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, "test_user", cfg.Database.User)
	assert.Equal(t, "test_password", cfg.Database.Password)
	assert.Equal(t, "test_db", cfg.Database.Name)

	// Проверяем значения по умолчанию
	assert.Equal(t, "deepseek-chat", cfg.AI.Model)
	assert.Equal(t, 1000, cfg.AI.MaxTokens)
	assert.Equal(t, 0.7, cfg.AI.Temperature)
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "disable", cfg.Database.SSLMode)
	assert.Equal(t, "development", cfg.App.Env)
	assert.Equal(t, "info", cfg.App.LogLevel)
	assert.Equal(t, 8080, cfg.App.Port)
}

func TestDatabaseDSN(t *testing.T) {
	cfg := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "test_user",
		Password: "test_password",
		Name:     "test_db",
		SSLMode:  "disable",
	}

	dsn := cfg.GetDSN()
	expected := "host=localhost port=5432 user=test_user password=test_password dbname=test_db sslmode=disable"
	assert.Equal(t, expected, dsn)
}

func TestAppConfigMethods(t *testing.T) {
	cfg := &AppConfig{
		Env:      "development",
		LogLevel: "debug",
	}

	assert.True(t, cfg.IsDevelopment())
	assert.False(t, cfg.IsProduction())

	cfg.Env = "production"
	assert.False(t, cfg.IsDevelopment())
	assert.True(t, cfg.IsProduction())
}

func TestValidateConfig(t *testing.T) {
	// Тест с пустыми обязательными полями
	cfg := &Config{}
	err := validateConfig(cfg)
	assert.Error(t, err)

	// Тест с корректной конфигурацией
	cfg = &Config{
		Telegram: TelegramConfig{
			BotToken: "test_token",
		},
		AI: AIConfig{
			Provider: "openrouter",
			OpenRouter: OpenRouterConfig{
				APIKey: "test_key",
			},
		},
		Database: DatabaseConfig{
			Host:     "localhost",
			User:     "test_user",
			Password: "test_password",
			Name:     "test_db",
		},
		YooKassa: YooKassaConfig{
			ProviderToken: "test_provider_token",
		},
	}
	err = validateConfig(cfg)
	assert.NoError(t, err)
}
