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

type OpenRouterClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     *zap.Logger
	siteURL    string
	siteName   string
}

func NewOpenRouterClient(apiKey, siteURL, siteName string, logger *zap.Logger) *OpenRouterClient {
	return &OpenRouterClient{
		baseURL:  "https://openrouter.ai/api/v1",
		apiKey:   apiKey,
		siteURL:  siteURL,
		siteName: siteName,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

type OpenRouterRequest struct {
	Model       string                   `json:"model"`
	Messages    []OpenRouterMessage      `json:"messages"`
	Temperature *float64                 `json:"temperature,omitempty"`
	MaxTokens   *int                     `json:"max_tokens,omitempty"`
	Stream      bool                     `json:"stream,omitempty"`
}

type OpenRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenRouterResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []OpenRouterChoice     `json:"choices"`
	Usage   OpenRouterUsage        `json:"usage"`
}

type OpenRouterChoice struct {
	Index        int                   `json:"index"`
	Message      OpenRouterMessage     `json:"message"`
	FinishReason string                `json:"finish_reason"`
}

type OpenRouterUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenRouterError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func (c *OpenRouterClient) GenerateResponse(ctx context.Context, messages []Message, options GenerationOptions) (*Response, error) {
	// Преобразуем сообщения в формат OpenRouter
	openRouterMessages := make([]OpenRouterMessage, len(messages))
	for i, msg := range messages {
		openRouterMessages[i] = OpenRouterMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Создаем запрос
	request := OpenRouterRequest{
		Model:    "deepseek/deepseek-r1-0528:free", // Используем бесплатную модель DeepSeek
		Messages: openRouterMessages,
		Stream:   false,
	}

	// Добавляем опциональные параметры
	if options.Temperature > 0 {
		request.Temperature = &options.Temperature
	}
	if options.MaxTokens > 0 {
		request.MaxTokens = &options.MaxTokens
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	c.logger.Debug("отправляем запрос к OpenRouter",
		zap.String("model", request.Model),
		zap.Int("messages_count", len(messages)),
		zap.Any("options", options))

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	// Устанавливаем заголовки
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	
	// Добавляем опциональные заголовки для рейтинга на openrouter.ai
	if c.siteURL != "" {
		req.Header.Set("HTTP-Referer", c.siteURL)
	}
	if c.siteName != "" {
		req.Header.Set("X-Title", c.siteName)
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка отправки запроса к OpenRouter: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	duration := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		c.logger.Error("ошибка API OpenRouter",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response_body", string(body)))

		var openRouterErr OpenRouterError
		if err := json.Unmarshal(body, &openRouterErr); err != nil {
			return nil, fmt.Errorf("ошибка OpenRouter API (статус %d): %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("ошибка OpenRouter API: %s", openRouterErr.Error.Message)
	}

	var openRouterResp OpenRouterResponse
	if err := json.Unmarshal(body, &openRouterResp); err != nil {
		return nil, fmt.Errorf("ошибка десериализации ответа: %w", err)
	}

	if len(openRouterResp.Choices) == 0 {
		return nil, fmt.Errorf("пустой ответ от OpenRouter")
	}

	content := openRouterResp.Choices[0].Message.Content

	c.logger.Info("получен ответ от OpenRouter",
		zap.String("model", openRouterResp.Model),
		zap.Int("prompt_tokens", openRouterResp.Usage.PromptTokens),
		zap.Int("completion_tokens", openRouterResp.Usage.CompletionTokens),
		zap.Int("total_tokens", openRouterResp.Usage.TotalTokens),
		zap.Duration("duration", duration),
		zap.Int("content_length", len(content)))

	return &Response{
		Content: content,
		Usage: Usage{
			PromptTokens:     openRouterResp.Usage.PromptTokens,
			CompletionTokens: openRouterResp.Usage.CompletionTokens,
			TotalTokens:      openRouterResp.Usage.TotalTokens,
		},
		Provider: "OpenRouter/DeepSeek",
	}, nil
}

func (c *OpenRouterClient) GetName() string {
	return "OpenRouter"
}
