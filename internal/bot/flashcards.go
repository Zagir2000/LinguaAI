package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lingua-ai/internal/flashcards"
	"lingua-ai/pkg/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

// FlashcardHandler обработчик команд для словарных карточек
type FlashcardHandler struct {
	bot              *tgbotapi.BotAPI
	flashcardService *flashcards.Service
	logger           *zap.Logger
}

// NewFlashcardHandler создает новый обработчик карточек
func NewFlashcardHandler(bot *tgbotapi.BotAPI, flashcardService *flashcards.Service, logger *zap.Logger) *FlashcardHandler {
	return &FlashcardHandler{
		bot:              bot,
		flashcardService: flashcardService,
		logger:           logger,
	}
}

// HandleFlashcardsCommand обрабатывает команду /flashcards
func (h *FlashcardHandler) HandleFlashcardsCommand(ctx context.Context, chatID int64, userID int64, userLevel string) error {
	// Проверяем, есть ли активная сессия
	session := h.flashcardService.GetCurrentSession(userID)
	if session != nil {
		return h.showCurrentCard(ctx, chatID, userID)
	}

	// Получаем рекомендацию по времени изучения
	recommendation, err := h.flashcardService.GetRecommendedStudyTime(ctx, userID)
	if err != nil {
		h.logger.Error("ошибка получения рекомендации", zap.Error(err))
		recommendation = "Готовы изучать новые слова?"
	}

	// Показываем меню карточек
	messageText := fmt.Sprintf(`📚 <b>Словарные карточки</b>

%s

Выберите действие:`, recommendation)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎯 Начать изучение", "flashcard_start"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📊 Моя статистика", "flashcard_stats"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Назад", "flashcard_back"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, messageText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err = h.bot.Send(msg)
	return err
}

// HandleFlashcardCallback обрабатывает callback от inline кнопок
func (h *FlashcardHandler) HandleFlashcardCallback(ctx context.Context, callback *tgbotapi.CallbackQuery, userID int64, userLevel string) error {
	data := callback.Data
	chatID := callback.Message.Chat.ID

	h.logger.Debug("обработка flashcard callback",
		zap.String("data", data),
		zap.Int64("user_id", userID))

	switch {
	case data == "flashcard_start":
		return h.startFlashcardSession(ctx, chatID, userID, userLevel)
	case data == "flashcard_stats":
		return h.showFlashcardStats(ctx, chatID, userID)
	case data == "flashcard_back":
		return h.showMainMenu(ctx, chatID)
	case data == "flashcard_show_translation":
		return h.handleShowTranslation(ctx, callback, userID)
	case strings.HasPrefix(data, "flashcard_answer_"):
		return h.handleCardAnswer(ctx, callback, userID)
	case data == "flashcard_next":
		return h.showCurrentCard(ctx, chatID, userID)
	case data == "flashcard_end":
		return h.endFlashcardSession(ctx, chatID, userID)
	case data == "flashcard_results":
		session := h.flashcardService.GetCurrentSession(userID)
		if session != nil {
			return h.showSessionResults(ctx, chatID, userID, session)
		}
		return h.HandleFlashcardsCommand(ctx, chatID, userID, userLevel)
	default:
		return fmt.Errorf("неизвестная команда карточек: %s", data)
	}
}

// startFlashcardSession начинает новую сессию изучения
func (h *FlashcardHandler) startFlashcardSession(ctx context.Context, chatID int64, userID int64, userLevel string) error {
	session, err := h.flashcardService.StartFlashcardSession(ctx, userID, userLevel)
	if err != nil {
		h.logger.Error("ошибка начала сессии карточек", zap.Error(err))
		return h.sendMessage(chatID, "❌ Ошибка начала изучения. Попробуйте позже.")
	}

	// Проверяем, что сессия создана корректно
	if session == nil {
		recommendedTime, timeErr := h.flashcardService.GetRecommendedStudyTime(ctx, userID)
		if timeErr == nil && recommendedTime != "" {
			return h.sendMessage(chatID, fmt.Sprintf("🎉 <b>Все карточки повторены!</b>\n\n%s\n\n⏰ Карточки станут доступны для повторения через некоторое время согласно алгоритму интервального повторения.", recommendedTime))
		}

		return h.sendMessage(chatID, "🎉 <b>Все карточки повторены!</b>\n\n⏰ Карточки станут доступны для повторения через некоторое время согласно алгоритму интервального повторения.\n\nПопробуйте позже или добавьте новые слова для изучения!")
	}

	if session.CurrentCard == nil {
		return h.sendMessage(chatID, "🎉 Отлично! У вас нет карточек для повторения. Проверьте завтра!")
	}

	// Показываем первую карточку
	return h.showCurrentCard(ctx, chatID, userID)
}

// showCurrentCard показывает текущую карточку
func (h *FlashcardHandler) showCurrentCard(ctx context.Context, chatID int64, userID int64) error {
	session := h.flashcardService.GetCurrentSession(userID)
	if session == nil {
		h.logger.Warn("активная сессия не найдена при показе карточки", zap.Int64("user_id", userID))
		return h.sendMessage(chatID, "❌ Активная сессия не найдена.\n\nПопробуйте начать изучение заново, нажав на кнопку \"📝 Словарные карточки\".")
	}

	if session.CurrentCard == nil {
		return h.showSessionResults(ctx, chatID, userID, session)
	}

	card := session.CurrentCard.Flashcard
	progress := h.flashcardService.GetSessionProgress(userID)

	messageText := fmt.Sprintf(`📚 <b>Карточка %d/%d</b>

🇬🇧 <b>%s</b>

<i>%s</i>

💡 Знаете перевод этого слова?`,
		progress["completed"].(int)+1,
		progress["total_cards"].(int),
		card.Word,
		card.Example,
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("👀 Показать перевод", "flashcard_show_translation"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❌ Завершить", "flashcard_end"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, messageText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err := h.bot.Send(msg)
	return err
}

// handleShowTranslation показывает перевод и варианты ответа (редактирует сообщение)
func (h *FlashcardHandler) handleShowTranslation(ctx context.Context, callback *tgbotapi.CallbackQuery, userID int64) error {
	chatID := callback.Message.Chat.ID
	session := h.flashcardService.GetCurrentSession(userID)
	if session == nil || session.CurrentCard == nil {
		h.logger.Warn("сессия или карточка не найдена при показе перевода",
			zap.Int64("user_id", userID),
			zap.Bool("session_exists", session != nil))
		return h.sendMessage(chatID, "❌ Активная карточка не найдена.\n\nПопробуйте начать изучение заново, нажав на кнопку \"📝 Словарные карточки\".")
	}

	card := session.CurrentCard.Flashcard
	progress := h.flashcardService.GetSessionProgress(userID)

	messageText := fmt.Sprintf(`📚 <b>Карточка %d/%d</b>

🇬🇧 <b>%s</b>
🇷🇺 <b>%s</b>

<i>%s</i>

❓ Насколько хорошо вы знали это слово?`,
		progress["completed"].(int)+1,
		progress["total_cards"].(int),
		card.Word,
		card.Translation,
		card.Example,
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("😊 Легко", "flashcard_answer_easy"),
			tgbotapi.NewInlineKeyboardButtonData("🤔 Хорошо", "flashcard_answer_good"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("😓 Сложно", "flashcard_answer_hard"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Не знал", "flashcard_answer_wrong"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, messageText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err := h.bot.Send(msg)
	return err
}

// handleCardAnswer обрабатывает ответ пользователя на карточку
func (h *FlashcardHandler) handleCardAnswer(ctx context.Context, callback *tgbotapi.CallbackQuery, userID int64) error {
	data := callback.Data
	chatID := callback.Message.Chat.ID

	var isCorrect bool
	var difficulty int

	switch data {
	case "flashcard_answer_easy":
		isCorrect = true
		difficulty = 1
	case "flashcard_answer_good":
		isCorrect = true
		difficulty = 3
	case "flashcard_answer_hard":
		isCorrect = true
		difficulty = 4
	case "flashcard_answer_wrong":
		isCorrect = false
		difficulty = 5
	default:
		return fmt.Errorf("неизвестный ответ: %s", data)
	}

	answer, err := h.flashcardService.AnswerCard(ctx, userID, isCorrect, difficulty)
	if err != nil {
		h.logger.Error("ошибка обработки ответа", zap.Error(err))

		// Если сессия потеряна, попробуем восстановить её
		if err.Error() == "активная сессия не найдена" {
			h.logger.Info("попытка восстановления сессии карточек", zap.Int64("user_id", userID))
			return h.sendMessage(chatID, "❌ Активная карточка не найдена.\n\nПопробуйте начать изучение заново, нажав на кнопку \"📝 Словарные карточки\".")
		}

		return h.sendMessage(chatID, "❌ Ошибка обработки ответа.")
	}

	// Показываем результат ответа
	var resultEmoji string
	var nextReviewText string

	if answer.IsCorrect {
		resultEmoji = "✅"
		hours := int(answer.NextReviewIn.Hours())
		if hours < 1 {
			nextReviewText = "Повторим скоро"
		} else {
			nextReviewText = fmt.Sprintf("Повторим через %d ч", hours)
		}
	} else {
		resultEmoji = "❌"
		nextReviewText = "Повторим скоро"
	}

	// Проверяем, есть ли еще карточки
	session := h.flashcardService.GetCurrentSession(userID)
	hasMoreCards := session != nil && session.CurrentCard != nil

	messageText := fmt.Sprintf(`%s <b>Ответ записан!</b>

%s

%s`,
		resultEmoji,
		nextReviewText,
		func() string {
			if hasMoreCards {
				return "Переходим к следующей карточке..."
			}
			return "🎉 Сессия завершена!"
		}(),
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			func() tgbotapi.InlineKeyboardButton {
				if hasMoreCards {
					return tgbotapi.NewInlineKeyboardButtonData("➡️ Следующая", "flashcard_next")
				}
				return tgbotapi.NewInlineKeyboardButtonData("📊 Результаты", "flashcard_results")
			}(),
		),
	)

	// Редактируем сообщение
	editMsg := tgbotapi.NewEditMessageText(chatID, callback.Message.MessageID, messageText)
	editMsg.ParseMode = "HTML"
	editMsg.ReplyMarkup = &keyboard

	_, err = h.bot.Send(editMsg)
	return err
}

// showSessionResults показывает результаты сессии
func (h *FlashcardHandler) showSessionResults(ctx context.Context, chatID int64, userID int64, session *models.FlashcardSession) error {
	accuracy := float64(session.CorrectAnswers) / float64(session.CardsCompleted) * 100
	if session.CardsCompleted == 0 {
		accuracy = 0
	}

	messageText := fmt.Sprintf(`🎉 <b>Сессия завершена!</b>

📊 <b>Результаты:</b>
• Изучено карточек: %d
• Правильных ответов: %d
• Точность: %.1f%%
• Время изучения: %d мин

🌟 Отличная работа! Продолжайте в том же духе!`,
		session.CardsCompleted,
		session.CorrectAnswers,
		accuracy,
		int(time.Since(session.SessionStarted).Minutes()),
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Еще раз", "flashcard_start"),
			tgbotapi.NewInlineKeyboardButtonData("📊 Статистика", "flashcard_stats"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏠 Главное меню", "flashcard_back"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, messageText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err := h.bot.Send(msg)

	// Завершаем сессию
	h.flashcardService.EndSession(userID)

	return err
}

// showFlashcardStats показывает статистику пользователя
func (h *FlashcardHandler) showFlashcardStats(ctx context.Context, chatID int64, userID int64) error {
	stats, err := h.flashcardService.GetUserStats(ctx, userID)
	if err != nil {
		h.logger.Error("ошибка получения статистики карточек", zap.Error(err))
		return h.sendMessage(chatID, "❌ Ошибка получения статистики.")
	}

	totalCards := stats["total_cards"].(int)
	learnedCards := stats["learned_cards"].(int)
	cardsToReview := stats["cards_to_review"].(int)
	accuracy := stats["accuracy_percentage"].(float64)

	messageText := fmt.Sprintf(`📊 <b>Статистика карточек</b>

📚 <b>Общее:</b>
• Всего карточек: %d
• Выучено слов: %d
• К повторению: %d
• Точность ответов: %.1f%%

📈 <b>Прогресс:</b>
%s

%s`,
		totalCards,
		learnedCards,
		cardsToReview,
		accuracy,
		h.getProgressBar(learnedCards, totalCards),
		func() string {
			if cardsToReview > 0 {
				return fmt.Sprintf("🎯 Рекомендуем повторить %d карточек сегодня!", cardsToReview)
			}
			return "🎉 Все карточки повторены! Отличная работа!"
		}(),
	)

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎯 Начать изучение", "flashcard_start"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "flashcard_back"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, messageText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err = h.bot.Send(msg)
	return err
}

// endFlashcardSession завершает сессию карточек
func (h *FlashcardHandler) endFlashcardSession(ctx context.Context, chatID int64, userID int64) error {
	session := h.flashcardService.GetCurrentSession(userID)
	if session == nil {
		return h.HandleFlashcardsCommand(ctx, chatID, userID, "beginner") // Fallback
	}

	h.flashcardService.EndSession(userID)

	messageText := `📚 <b>Сессия завершена</b>

Вы можете продолжить изучение в любое время!`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔄 Продолжить", "flashcard_start"),
			tgbotapi.NewInlineKeyboardButtonData("🏠 Главное меню", "flashcard_back"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, messageText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err := h.bot.Send(msg)
	return err
}

// showMainMenu показывает главное меню
func (h *FlashcardHandler) showMainMenu(ctx context.Context, chatID int64) error {
	messageText := `🏠 <b>Главное меню</b>

Добро пожаловать в Lingua AI! 

Выберите действие:`

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("📚 Обучение", "learning_menu"),
			tgbotapi.NewInlineKeyboardButtonData("📊 Статистика", "main_stats"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🏆 Рейтинг", "main_rating"),
			tgbotapi.NewInlineKeyboardButtonData("💎 Премиум", "main_premium"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("❓ Помощь", "main_help"),
		),
	)

	msg := tgbotapi.NewMessage(chatID, messageText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err := h.bot.Send(msg)
	return err
}

// sendMessage отправляет простое текстовое сообщение
func (h *FlashcardHandler) sendMessage(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "HTML"
	_, err := h.bot.Send(msg)
	return err
}

// getProgressBar создает текстовый прогресс-бар
func (h *FlashcardHandler) getProgressBar(current, total int) string {
	if total == 0 {
		return "▱▱▱▱▱▱▱▱▱▱ 0%"
	}

	percentage := float64(current) / float64(total) * 100
	filled := int(percentage / 10)

	bar := ""
	for i := 0; i < 10; i++ {
		if i < filled {
			bar += "▰"
		} else {
			bar += "▱"
		}
	}

	return fmt.Sprintf("%s %.1f%%", bar, percentage)
}



