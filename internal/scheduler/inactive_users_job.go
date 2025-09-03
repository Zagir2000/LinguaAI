package scheduler

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"

	"lingua-ai/internal/ai"
	"lingua-ai/internal/message"
	"lingua-ai/internal/user"
	"lingua-ai/pkg/models"
)

// InactiveUsersJob отвечает за отправку заданий неактивным пользователям
type InactiveUsersJob struct {
	userService    *user.Service
	messageService *message.Service
	aiClient       ai.AIClient
	bot            *tgbotapi.BotAPI
	logger         *zap.Logger
}

// NewInactiveUsersJob создает новую джобу для неактивных пользователей
func NewInactiveUsersJob(
	userService *user.Service,
	messageService *message.Service,
	aiClient ai.AIClient,
	bot *tgbotapi.BotAPI,
	logger *zap.Logger,
) *InactiveUsersJob {
	return &InactiveUsersJob{
		userService:    userService,
		messageService: messageService,
		aiClient:       aiClient,
		bot:            bot,
		logger:         logger,
	}
}

// Run запускает джобу проверки неактивных пользователей
func (j *InactiveUsersJob) Run(ctx context.Context) error {
	j.logger.Info("запуск джобы проверки неактивных пользователей")

	// Получаем пользователей неактивных более 24 часов
	inactiveUsers, err := j.userService.GetInactiveUsers(ctx, 24*time.Hour)
	if err != nil {
		j.logger.Error("ошибка получения неактивных пользователей", zap.Error(err))
		return fmt.Errorf("ошибка получения неактивных пользователей: %w", err)
	}

	j.logger.Info("найдено неактивных пользователей", zap.Int("count", len(inactiveUsers)))

	// Отправляем задания каждому неактивному пользователю
	for _, user := range inactiveUsers {
		if err := j.sendTaskToUser(ctx, user); err != nil {
			j.logger.Error("ошибка отправки задания пользователю",
				zap.Error(err),
				zap.Int64("user_id", user.ID),
				zap.String("username", user.Username))
			continue
		}
	}

	j.logger.Info("джоба проверки неактивных пользователей завершена")
	return nil
}

// sendTaskToUser отправляет персонализированное задание пользователю
func (j *InactiveUsersJob) sendTaskToUser(ctx context.Context, user *models.User) error {
	// Генерируем задание на основе уровня пользователя
	task, err := j.generateTask(ctx, user)
	if err != nil {
		return fmt.Errorf("ошибка генерации задания: %w", err)
	}

	// Формируем сообщение
	messageText := fmt.Sprintf(`🎯 <b>Привет, %s!</b>

Давно не виделись! Вот интересное задание для тебя:

%s

💡 <i>Попробуй выполнить это задание и отправь мне свой ответ на английском!</i>

🔥 За активность ты получишь дополнительные XP!`, user.FirstName, task)

	// Сохраняем задание как системное сообщение в истории
	systemMessage := &models.CreateMessageRequest{
		UserID:  user.ID,
		Role:    "system",
		Content: fmt.Sprintf("Система отправила задание: %s", task),
	}

	if _, err := j.messageService.CreateMessage(ctx, systemMessage); err != nil {
		j.logger.Error("ошибка сохранения системного сообщения",
			zap.Error(err),
			zap.Int64("user_id", user.ID))
		// Продолжаем, даже если не удалось сохранить
	}

	// Отправляем сообщение с безопасной обработкой HTML
	msg := tgbotapi.NewMessage(user.TelegramID, messageText)
	msg.ParseMode = "HTML"

	_, err = j.bot.Send(msg)
	if err != nil {
		// Если HTML парсинг не удался, пробуем отправить как обычный текст
		j.logger.Warn("ошибка отправки HTML сообщения, отправляем как обычный текст",
			zap.Error(err),
			zap.Int64("user_id", user.ID))

		// Экранируем HTML и отправляем как обычный текст
		safeText := html.EscapeString(fmt.Sprintf(`🎯 Привет, %s!

Давно не виделись! Вот интересное задание для тебя:

%s

💡 Попробуй выполнить это задание и отправь мне свой ответ на английском!

🔥 За активность ты получишь дополнительные XP!`, user.FirstName, task))

		fallbackMsg := tgbotapi.NewMessage(user.TelegramID, safeText)
		_, fallbackErr := j.bot.Send(fallbackMsg)
		if fallbackErr != nil {
			return fmt.Errorf("ошибка отправки fallback сообщения: %w", fallbackErr)
		}
	}

	j.logger.Info("задание отправлено пользователю",
		zap.Int64("user_id", user.ID),
		zap.String("username", user.Username),
		zap.String("level", user.Level))

	return nil
}

// generateTask генерирует персонализированное задание на основе уровня пользователя
func (j *InactiveUsersJob) generateTask(ctx context.Context, user *models.User) (string, error) {
	// Получаем историю сообщений для контекста
	history, err := j.messageService.GetChatHistory(ctx, user.ID, 10) // Последние 10 сообщений
	if err != nil {
		j.logger.Error("ошибка получения истории сообщений",
			zap.Error(err),
			zap.Int64("user_id", user.ID))
		// Продолжаем без истории
	}

	// Определяем сложность задания на основе уровня
	var difficulty string
	switch user.Level {
	case models.LevelBeginner:
		difficulty = "beginner (A1-A2)"
	case models.LevelIntermediate:
		difficulty = "intermediate (B1-B2)"
	case models.LevelAdvanced:
		difficulty = "advanced (C1-C2)"
	default:
		difficulty = "beginner (A1-A2)"
	}

	// Собираем информацию о предыдущих заданиях
	var previousTasks []string
	for _, msg := range history.Messages {
		if msg.Role == "system" && strings.Contains(msg.Content, "Система отправила задание:") {
			taskPart := strings.TrimPrefix(msg.Content, "Система отправила задание: ")
			previousTasks = append(previousTasks, taskPart)
		}
	}

	// Формируем контекст для избежания повторов
	var contextInfo string
	if len(previousTasks) > 0 {
		contextInfo = fmt.Sprintf(`
ВАЖНО: Не повторяй эти задания, которые уже были отправлены:
%s

Придумай НОВОЕ, оригинальное задание, не похожее на предыдущие.`, strings.Join(previousTasks, "\n- "))
	}

	// Промпт для генерации задания
	prompt := fmt.Sprintf(`Ты — AI-преподаватель английского языка для Telegram-бота.

Задача: придумай короткое и мотивирующее задание по английскому для студента уровня %s.%s

ФОРМАТИРОВАНИЕ:
- 🚫 Никогда не используй Markdown (** или #)
- ✅ Используй только HTML-теги: <b>жирный</b>, <i>курсив</i>, <u>подчеркнутый</u>
- Текст должен корректно отображаться в Telegram
- Ответ всегда на английском языке

ТРЕБОВАНИЯ К ЗАДАНИЮ:
1. Интересное и мотивирующее упражнение
2. Подходит для уровня %s
3. Стимулирует ученика написать ответ на английском
4. Четкая инструкция
5. Дружелюбный стиль
6. Длина: максимум 2–3 предложения
Сгенерируй ОДНО конкретное задание прямо сейчас в этом формате:
<b>Task:</b> [твое задание]`, difficulty, contextInfo, difficulty)

	// Получаем ответ от AI
	options := ai.GenerationOptions{
		Temperature: 0.7,
		MaxTokens:   300,
	}

	response, err := j.aiClient.GenerateResponse(ctx, []ai.Message{
		{
			Role:    "user",
			Content: prompt,
		},
	}, options)

	if err != nil {
		j.logger.Error("ошибка генерации задания от AI", zap.Error(err))
		// Возвращаем дефолтное задание в случае ошибки
		return j.getDefaultTask(user.Level), nil
	}

	return response.Content, nil
}

// getDefaultTask возвращает дефолтное задание для уровня в случае ошибки AI
func (j *InactiveUsersJob) getDefaultTask(level string) string {
	defaultTasks := map[string][]string{
		models.LevelBeginner: {
			"Tell me about your favorite food in 2-3 sentences. What do you like about it?",
			"Describe your morning routine. What do you usually do when you wake up?",
			"What's your favorite color and why? Share your thoughts!",
		},
		models.LevelIntermediate: {
			"If you could visit any country in the world, where would you go and what would you do there?",
			"Describe a memorable moment from your childhood. What made it special?",
			"What's a skill you'd like to learn and why? How would it change your life?",
		},
		models.LevelAdvanced: {
			"What's your opinion on the impact of technology on human relationships? Share your perspective.",
			"Describe a book, movie, or article that changed your way of thinking. What insights did you gain?",
			"If you could solve one global problem, what would it be and how would you approach it?",
		},
	}

	tasks, exists := defaultTasks[level]
	if !exists {
		tasks = defaultTasks[models.LevelBeginner]
	}

	// Возвращаем случайное задание из списка
	return tasks[time.Now().Unix()%int64(len(tasks))]
}
