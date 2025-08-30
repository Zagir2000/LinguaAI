package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// DeepSeekClient клиент для работы с DeepSeek API
type DeepSeekClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewDeepSeekClient создает новый клиент DeepSeek
func NewDeepSeekClient(apiKey, baseURL string, logger *zap.Logger) *DeepSeekClient {
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1"
	}

	return &DeepSeekClient{
		apiKey:  apiKey,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}
}

// DeepSeekRequest представляет запрос к DeepSeek API
type DeepSeekRequest struct {
	Model       string            `json:"model"`
	Messages    []DeepSeekMessage `json:"messages"`
	Temperature float64           `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	Stream      bool              `json:"stream"`
}

// DeepSeekMessage представляет сообщение в формате DeepSeek
type DeepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// DeepSeekResponse представляет ответ от DeepSeek API
type DeepSeekResponse struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []DeepSeekChoice `json:"choices"`
	Usage   DeepSeekUsage    `json:"usage"`
}

// DeepSeekChoice представляет вариант ответа
type DeepSeekChoice struct {
	Index        int             `json:"index"`
	Message      DeepSeekMessage `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

// DeepSeekUsage представляет статистику использования токенов
type DeepSeekUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// GenerateResponse генерирует ответ через DeepSeek API
func (c *DeepSeekClient) GenerateResponse(ctx context.Context, messages []Message, options GenerationOptions) (*Response, error) {
	c.logger.Debug("отправляем запрос в DeepSeek",
		zap.Int("messages_count", len(messages)),
		zap.Float64("temperature", options.Temperature),
		zap.Int("max_tokens", options.MaxTokens))

	// Конвертируем сообщения в формат DeepSeek
	deepSeekMessages := make([]DeepSeekMessage, len(messages))
	for i, msg := range messages {
		deepSeekMessages[i] = DeepSeekMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Создаем запрос
	request := DeepSeekRequest{
		Model:       "deepseek-chat", // Используем основную модель DeepSeek
		Messages:    deepSeekMessages,
		Temperature: options.Temperature,
		MaxTokens:   options.MaxTokens,
		Stream:      false,
	}

	// Сериализуем запрос
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	// Устанавливаем заголовки
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// Отправляем запрос
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка отправки запроса: %w", err)
	}
	defer resp.Body.Close()

	// Читаем ответ
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		c.logger.Error("ошибка DeepSeek API",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(responseBody)))
		return nil, fmt.Errorf("ошибка DeepSeek API (статус %d): %s", resp.StatusCode, string(responseBody))
	}

	// Парсим ответ
	var deepSeekResp DeepSeekResponse
	err = json.Unmarshal(responseBody, &deepSeekResp)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	// Проверяем наличие вариантов ответа
	if len(deepSeekResp.Choices) == 0 {
		return nil, fmt.Errorf("нет вариантов ответа от DeepSeek")
	}

	choice := deepSeekResp.Choices[0]

	c.logger.Debug("получен ответ от DeepSeek",
		zap.String("model", deepSeekResp.Model),
		zap.Int("prompt_tokens", deepSeekResp.Usage.PromptTokens),
		zap.Int("completion_tokens", deepSeekResp.Usage.CompletionTokens),
		zap.String("finish_reason", choice.FinishReason))

	return &Response{
		Content: choice.Message.Content,
		Model:   deepSeekResp.Model,
		Usage: Usage{
			PromptTokens:     deepSeekResp.Usage.PromptTokens,
			CompletionTokens: deepSeekResp.Usage.CompletionTokens,
			TotalTokens:      deepSeekResp.Usage.TotalTokens,
		},
		FinishReason: choice.FinishReason,
		Provider:     "deepseek",
	}, nil
}

// GetName возвращает название провайдера
func (c *DeepSeekClient) GetName() string {
	return "DeepSeek"
}
