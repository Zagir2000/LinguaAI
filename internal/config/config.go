package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

// Config содержит все конфигурационные параметры приложения
type Config struct {
	Telegram TelegramConfig
	AI       AIConfig
	Whisper  WhisperConfig
	Database DatabaseConfig
	App      AppConfig
	YooKassa YooKassaConfig
}

// TelegramConfig содержит настройки Telegram бота
type TelegramConfig struct {
	BotToken   string
	WebhookURL string
}

// AIConfig содержит настройки AI провайдеров
type AIConfig struct {
	Provider    string
	Model       string
	MaxTokens   int
	Temperature float64
	DeepSeek    DeepSeekConfig
	OpenRouter  OpenRouterConfig
}

type DeepSeekConfig struct {
	APIKey  string
	BaseURL string
}

type OpenRouterConfig struct {
	APIKey   string
	SiteURL  string
	SiteName string
}

// WhisperConfig содержит настройки Whisper API
type WhisperConfig struct {
	APIURL string
}

type DatabaseConfig struct {
	Host          string
	Port          int
	User          string
	Password      string
	Name          string
	SSLMode       string
	MigrationPath string
}

type AppConfig struct {
	Env      string
	LogLevel string
	Port     int
}

// YooKassaConfig содержит настройки ЮKassa
type YooKassaConfig struct {
	ShopID        string
	SecretKey     string
	TestMode      bool
	ProviderToken string
}

// Load загружает конфигурацию из переменных окружения и .env
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}

	// Telegram
	cfg.Telegram.BotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	cfg.Telegram.WebhookURL = os.Getenv("TELEGRAM_WEBHOOK_URL")

	// AI
	cfg.AI.Provider = getEnvDefault("AI_PROVIDER", "deepseek")
	cfg.AI.Model = getEnvDefault("AI_MODEL", "deepseek-chat")
	cfg.AI.MaxTokens = getEnvIntDefault("AI_MAX_TOKENS", 1000)
	cfg.AI.Temperature = getEnvFloatDefault("AI_TEMPERATURE", 0.7)
	cfg.AI.DeepSeek.APIKey = os.Getenv("DEEPSEEK_API_KEY")
	cfg.AI.DeepSeek.BaseURL = getEnvDefault("DEEPSEEK_BASE_URL", "https://api.deepseek.com/v1")
	cfg.AI.OpenRouter.APIKey = os.Getenv("OPENROUTER_API_KEY")
	cfg.AI.OpenRouter.SiteURL = getEnvDefault("OPENROUTER_SITE_URL", "https://lingua-ai.ru")
	cfg.AI.OpenRouter.SiteName = getEnvDefault("OPENROUTER_SITE_NAME", "Lingua AI")

	// Whisper
	cfg.Whisper.APIURL = getEnvDefault("WHISPER_API_URL", "http://whisper:8080")

	// Database
	cfg.Database.Host = getEnvDefault("DB_HOST", "localhost")
	cfg.Database.Port = getEnvIntDefault("DB_PORT", 5432)
	cfg.Database.User = os.Getenv("DB_USER")
	cfg.Database.Password = os.Getenv("DB_PASSWORD")
	cfg.Database.Name = os.Getenv("DB_NAME")
	cfg.Database.SSLMode = getEnvDefault("DB_SSL_MODE", "disable")
	cfg.Database.MigrationPath = getEnvDefault("MIGRATION_PATH", "scripts/migrations")

	// YooKassa
	cfg.YooKassa.ShopID = getEnvDefault("YUKASSA_SHOP_ID", "test_shop_id")
	cfg.YooKassa.SecretKey = getEnvDefault("YUKASSA_SECRET_KEY", "test_secret_key")
	cfg.YooKassa.TestMode = getEnvBoolDefault("YUKASSA_TEST_MODE", true)
	cfg.YooKassa.ProviderToken = os.Getenv("YUKASSA_PROVIDER_TOKEN")

	// App
	cfg.App.Env = getEnvDefault("APP_ENV", "development")
	cfg.App.LogLevel = getEnvDefault("LOG_LEVEL", "info")
	cfg.App.Port = getEnvIntDefault("APP_PORT", 8080)

	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("ошибка валидации конфигурации: %w", err)
	}

	return cfg, nil
}

func getEnvDefault(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func getEnvIntDefault(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}

func getEnvFloatDefault(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func getEnvBoolDefault(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// validateConfig проверяет корректность конфигурации
func validateConfig(config *Config) error {
	if config.Telegram.BotToken == "" {
		return fmt.Errorf("TELEGRAM_BOT_TOKEN не установлен")
	}
	if config.AI.Provider == "deepseek" && config.AI.DeepSeek.APIKey == "" {
		return fmt.Errorf("DEEPSEEK_API_KEY не установлен")
	}
	if config.AI.Provider == "openrouter" && config.AI.OpenRouter.APIKey == "" {
		return fmt.Errorf("OPENROUTER_API_KEY не установлен")
	}
	if config.AI.Provider != "deepseek" && config.AI.Provider != "openrouter" {
		return fmt.Errorf("поддерживаются только AI_PROVIDER: deepseek, openrouter")
	}
	if config.Database.Host == "" {
		return fmt.Errorf("DB_HOST не установлен")
	}
	if config.Database.User == "" {
		return fmt.Errorf("DB_USER не установлен")
	}
	if config.Database.Password == "" {
		return fmt.Errorf("DB_PASSWORD не установлен")
	}
	if config.Database.Name == "" {
		return fmt.Errorf("DB_NAME не установлен")
	}
	if config.YooKassa.ProviderToken == "" {
		return fmt.Errorf("YUKASSA_PROVIDER_TOKEN не установлен")
	}

	return nil
}

// GetDSN возвращает строку подключения к базе данных
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}

// IsDevelopment проверяет, запущено ли приложение в режиме разработки
func (c *AppConfig) IsDevelopment() bool {
	return c.Env == "development"
}

// IsProduction проверяет, запущено ли приложение в продакшн режиме
func (c *AppConfig) IsProduction() bool {
	return c.Env == "production"
}

// GetLogLevel возвращает уровень логирования в формате zap
func (c *AppConfig) GetLogLevel() zap.AtomicLevel {
	switch c.LogLevel {
	case "debug":
		return zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		return zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		return zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		return zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		return zap.NewAtomicLevelAt(zap.InfoLevel)
	}
}
