package ai

import (
	"fmt"

	"go.uber.org/zap"
)

// NewAIClient создает новый AI клиент на основе конфигурации
func NewAIClient(cfg *AIConfig, logger *zap.Logger) (AIClient, error) {
	switch cfg.Provider {
	case "deepseek":
		return NewDeepSeekClient(cfg.DeepSeek.APIKey, cfg.DeepSeek.BaseURL, logger), nil
	case "openrouter":
		return NewOpenRouterClient(cfg.OpenRouter.APIKey, cfg.OpenRouter.SiteURL, cfg.OpenRouter.SiteName, logger), nil
	default:
		return nil, fmt.Errorf("неподдерживаемый AI провайдер: %s. Поддерживаются: 'deepseek', 'openrouter'", cfg.Provider)
	}
}
