package ai

import (
	"context"
	"html"
	"regexp"
	"strings"
)

// Message представляет сообщение для AI
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Response представляет ответ от AI
type Response struct {
	Content      string `json:"content"`
	Model        string `json:"model"`
	Usage        Usage  `json:"usage"`
	FinishReason string `json:"finish_reason"`
	Provider     string `json:"provider"`
}

// Usage представляет статистику использования токенов
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// GenerationOptions опции для генерации ответа
type GenerationOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

// AIClient интерфейс для работы с AI провайдерами
type AIClient interface {
	// GenerateResponse генерирует ответ на основе сообщений
	GenerateResponse(ctx context.Context, messages []Message, options GenerationOptions) (*Response, error)

	// GetName возвращает название провайдера
	GetName() string
}

// AIConfig содержит конфигурацию для AI клиентов
type AIConfig struct {
	Provider    string
	Model       string
	MaxTokens   int
	Temperature float64
	DeepSeek    DeepSeekConfig
	OpenRouter  OpenRouterConfig
}

// DeepSeekConfig конфигурация DeepSeek
type DeepSeekConfig struct {
	APIKey  string
	BaseURL string
}

// OpenRouterConfig конфигурация OpenRouter
type OpenRouterConfig struct {
	APIKey   string
	SiteURL  string
	SiteName string
}

// SystemPrompt возвращает базовый системный промпт для AI
func GetSystemPrompt() string {
	return `Преподаватель английского "Lingua AI".

СТРОГО: проверяй английские упражнения точно. Анализируй каждое слово.
НЕ придумывай новые задания если не просят.
НЕ задавай вопросы на русском.

Задача: проверка грамматики, исправление ошибок.
	Формат: HTML <b>жирный</b> <i>курсив</i>. Будь конкретным и точным.`
}

// fixHTMLTags исправляет некорректную HTML разметку и защищает от XSS
func fixHTMLTags(text string) string {
	// Разрешенные HTML теги для Telegram
	allowedTags := []string{"b", "i", "u", "code", "pre", "a", "strong", "em"}

	// Создаем regex для поиска всех HTML тегов
	htmlTagRegex := regexp.MustCompile(`</?([a-zA-Z][a-zA-Z0-9]*)\b[^>]*>`)

	// Заменяем все теги на безопасные версии
	result := htmlTagRegex.ReplaceAllStringFunc(text, func(tag string) string {
		// Извлекаем название тега
		tagNameRegex := regexp.MustCompile(`</?([a-zA-Z][a-zA-Z0-9]*)\b`)
		matches := tagNameRegex.FindStringSubmatch(tag)
		if len(matches) < 2 {
			return "" // Удаляем некорректные теги
		}

		tagName := strings.ToLower(matches[1])

		// Проверяем, разрешен ли тег
		allowed := false
		for _, allowedTag := range allowedTags {
			if tagName == allowedTag {
				allowed = true
				break
			}
		}

		if !allowed {
			return "" // Удаляем неразрешенные теги
		}

		// Возвращаем только основные теги без атрибутов (кроме <a>)
		if strings.HasPrefix(tag, "</") {
			return "</" + tagName + ">"
		} else if tagName == "a" {
			// Для ссылок оставляем href атрибут, но санитизируем его
			hrefRegex := regexp.MustCompile(`href\s*=\s*["']([^"']*)["']`)
			hrefMatches := hrefRegex.FindStringSubmatch(tag)
			if len(hrefMatches) > 1 {
				// Простая проверка URL (только http/https)
				href := hrefMatches[1]
				if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
					return `<a href="` + html.EscapeString(href) + `">`
				}
			}
			return "" // Удаляем ссылки без корректного href
		} else {
			return "<" + tagName + ">"
		}
	})

	// Экранируем оставшиеся опасные символы, но сохраняем разрешенные HTML теги
	// Сначала заменяем разрешенные теги на маркеры
	tagMarkers := map[string]string{
		"<b>": "{{BOLD_START}}", "</b>": "{{BOLD_END}}",
		"<i>": "{{ITALIC_START}}", "</i>": "{{ITALIC_END}}",
		"<u>": "{{UNDERLINE_START}}", "</u>": "{{UNDERLINE_END}}",
		"<code>": "{{CODE_START}}", "</code>": "{{CODE_END}}",
		"<strong>": "{{STRONG_START}}", "</strong>": "{{STRONG_END}}",
		"<em>": "{{EM_START}}", "</em>": "{{EM_END}}",
	}

	for tag, marker := range tagMarkers {
		result = strings.ReplaceAll(result, tag, marker)
	}

	// Экранируем остальные HTML символы
	result = html.EscapeString(result)

	// Возвращаем разрешенные теги
	for tag, marker := range tagMarkers {
		result = strings.ReplaceAll(result, marker, tag)
	}

	return result
}

// SanitizeResponse фильтрует ответ AI от упоминаний моделей и нерелевантных тем
func SanitizeResponse(text string) string {
	blockedPhrases := []string{
		"gpt-4", "gpt-3", "gpt", "chatgpt", "openai", "gigachat", "yandex", "сбер",
		"нейросеть", "модель", "языковая модель", "искусственный интеллект", "ai model",
		"large language model", "llm", "claude", "bard", "gemini", "anthropic",
		"я обучен", "меня обучили", "моя модель", "я основан на", "я создан",
		"я работаю на", "моя архитектура", "мои параметры", "i was trained",
		"trained on", "i'm trained", "i am trained", "was trained",
	}

	// Блокированные нерелевантные темы (если ответ начинается с обсуждения этих тем)
	irrelevantTopics := []string{
		"боевики", "фильмы", "кино", "актеры", "режиссеры", "сериалы", "мультфильмы",
		"политика", "новости", "спорт", "еда", "кулинария", "путешествия",
		"музыка", "певцы", "концерты", "игры", "программирование", "технологии",
		"автомобили", "мода", "красота", "медицина", "здоровье", "психология",
	}

	lower := strings.ToLower(text)

	// Проверяем упоминания моделей
	for _, phrase := range blockedPhrases {
		if strings.Contains(lower, phrase) {
			return "🤖 Я здесь, чтобы помочь с английским! Давай сосредоточимся на изучении языка. Что бы ты хотел изучить сегодня?"
		}
	}

	// Проверяем, не начинается ли ответ с нерелевантной темы
	words := strings.Fields(lower)
	if len(words) > 5 { // Проверяем только если ответ достаточно длинный
		firstWords := strings.Join(words[:5], " ")
		for _, topic := range irrelevantTopics {
			if strings.Contains(firstWords, topic) {
				return `Я помогаю изучать английский язык! 🇬🇧 
				
Если тебя интересует эта тема, давай изучим связанные с ней английские слова и фразы! 

Напиши мне, какие английские слова или грамматику ты хочешь изучить. 📚`
			}
		}
	}

	// Исправляем HTML теги и защищаем от XSS перед возвратом
	return fixHTMLTags(text)
}
