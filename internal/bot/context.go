package bot

import (
	"strings"
	"time"
)

// DialogContext содержит контекст диалога с пользователем
type DialogContext struct {
	UserID       int64
	Level        string
	SystemPrompt string
	Messages     []DialogMessage
	LastActivity time.Time
}

// DialogMessage представляет сообщение в диалоге
type DialogMessage struct {
	Role      string    // "user" или "assistant"
	Content   string    // содержимое сообщения
	Timestamp time.Time // время отправки
}

// NewDialogContext создает новый контекст диалога
func NewDialogContext(userID int64, level string, systemPrompt string) *DialogContext {
	return &DialogContext{
		UserID:       userID,
		Level:        level,
		SystemPrompt: systemPrompt,
		Messages:     make([]DialogMessage, 0),
		LastActivity: time.Now(),
	}
}

// AddUserMessage добавляет сообщение пользователя в контекст
func (dc *DialogContext) AddUserMessage(content string) {
	dc.Messages = append(dc.Messages, DialogMessage{
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	})
	dc.LastActivity = time.Now()
}

// AddAssistantMessage добавляет ответ ассистента в контекст
func (dc *DialogContext) AddAssistantMessage(content string) {
	dc.Messages = append(dc.Messages, DialogMessage{
		Role:      "assistant",
		Content:   content,
		Timestamp: time.Now(),
	})
	dc.LastActivity = time.Now()
}

// BuildFullPrompt строит полный промпт для AI с учетом контекста
func (dc *DialogContext) BuildFullPrompt() string {
	var prompt strings.Builder

	// Добавляем системный промпт
	prompt.WriteString(dc.SystemPrompt)
	prompt.WriteString("\n\n")

	// Добавляем историю диалога (последние 10 сообщений для экономии токенов)
	start := 0
	if len(dc.Messages) > 10 {
		start = len(dc.Messages) - 10
	}

	for i := start; i < len(dc.Messages); i++ {
		msg := dc.Messages[i]
		if msg.Role == "user" {
			prompt.WriteString("User: " + msg.Content + "\n")
		} else {
			prompt.WriteString("Assistant: " + msg.Content + "\n")
		}
	}

	return prompt.String()
}

// IsStale проверяет, не устарел ли контекст (больше 1 часа)
func (dc *DialogContext) IsStale() bool {
	return time.Since(dc.LastActivity) > time.Hour
}

// ClearHistory очищает историю сообщений, оставляя системный промпт
func (dc *DialogContext) ClearHistory() {
	dc.Messages = make([]DialogMessage, 0)
	dc.LastActivity = time.Now()
}
