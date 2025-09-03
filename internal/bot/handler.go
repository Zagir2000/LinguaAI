package bot

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"lingua-ai/internal/premium"
	"lingua-ai/internal/store"
	"lingua-ai/internal/tts"

	"lingua-ai/internal/ai"
	"lingua-ai/internal/flashcards"
	"lingua-ai/internal/message"
	"lingua-ai/internal/metrics"
	"lingua-ai/internal/referral"
	"lingua-ai/internal/user"
	"lingua-ai/internal/whisper"
	"lingua-ai/pkg/models"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

const (
	// Оптимальные значения для истории сообщений
	ChatHistoryForTranslation  = 5  // Для поиска переводов
	ChatHistoryForConversation = 10 // Для обычного общения
	ChatHistoryForAudio        = 8  // Для аудио обработки

	// Лимиты безопасности
	MaxFileSize       = 25 * 1024 * 1024 // 25MB максимум для аудио файлов
	MaxTextLength     = 4000             // Максимальная длина текста сообщения
	MaxUsernameLength = 32               // Максимальная длина username

	// Rate limiting
	MaxRequestsPerMinute = 30 // Максимум запросов в минуту на пользователя
	RateLimitWindow      = time.Minute
)

// RateLimiter простой rate limiter для пользователей
type RateLimiter struct {
	requests map[int64][]time.Time
	mutex    sync.RWMutex
}

// NewRateLimiter создает новый rate limiter
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		requests: make(map[int64][]time.Time),
	}
}

// IsAllowed проверяет, разрешен ли запрос для пользователя
func (rl *RateLimiter) IsAllowed(userID int64) bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()
	userRequests := rl.requests[userID]

	// Удаляем старые запросы
	var validRequests []time.Time
	for _, reqTime := range userRequests {
		if now.Sub(reqTime) < RateLimitWindow {
			validRequests = append(validRequests, reqTime)
		}
	}

	// Проверяем лимит
	if len(validRequests) >= MaxRequestsPerMinute {
		rl.requests[userID] = validRequests
		return false
	}

	// Добавляем текущий запрос
	validRequests = append(validRequests, now)
	rl.requests[userID] = validRequests
	return true
}

// Handler представляет обработчик сообщений Telegram
type Handler struct {
	bot              *tgbotapi.BotAPI
	userService      *user.Service
	messageService   *message.Service
	aiClient         ai.AIClient
	whisperClient    *whisper.Client
	ttsService       tts.TTSService
	messages         *Messages
	logger           *zap.Logger
	userMetrics      *metrics.Metrics
	aiMetrics        *metrics.Metrics
	activeLevelTests map[int64]*models.LevelTest // Хранилище активных тестов
	prompts          *SystemPrompts
	dialogContexts   map[int64]*DialogContext // контекст диалога для каждого пользователя
	premiumService   *premium.Service         // сервис премиум-подписки
	referralService  *referral.Service        // сервис реферальной системы
	rateLimiter      *RateLimiter             // rate limiter для защиты от спама
	flashcardHandler *FlashcardHandler        // обработчик словарных карточек
	store            store.Store              // хранилище для доступа к payment repo
}

// NewHandler создает новый обработчик
func NewHandler(
	bot *tgbotapi.BotAPI,
	userService *user.Service,
	messageService *message.Service,
	aiClient ai.AIClient,
	whisperClient *whisper.Client,
	ttsService tts.TTSService,
	logger *zap.Logger,
	userMetrics *metrics.Metrics,
	aiMetrics *metrics.Metrics,
	premiumService *premium.Service,
	referralService *referral.Service,
	flashcardService *flashcards.Service,
	store store.Store,
) *Handler {
	handler := &Handler{
		bot:              bot,
		userService:      userService,
		messageService:   messageService,
		aiClient:         aiClient,
		whisperClient:    whisperClient,
		ttsService:       ttsService,
		messages:         NewMessages(),
		logger:           logger,
		userMetrics:      userMetrics,
		aiMetrics:        aiMetrics,
		activeLevelTests: make(map[int64]*models.LevelTest),
		prompts:          NewSystemPrompts(),
		dialogContexts:   make(map[int64]*DialogContext),
		premiumService:   premiumService,
		referralService:  referralService,
		rateLimiter:      NewRateLimiter(),
		store:            store,
	}

	// Инициализируем обработчик карточек
	handler.flashcardHandler = NewFlashcardHandler(bot, flashcardService, logger)

	return handler
}

// HandleUpdate обрабатывает входящее обновление
func (h *Handler) HandleUpdate(ctx context.Context, update tgbotapi.Update) error {
	// Получаем ID пользователя для rate limiting
	var userID int64
	if update.Message != nil {
		userID = update.Message.From.ID
	} else if update.CallbackQuery != nil {
		userID = update.CallbackQuery.From.ID
	}

	// Проверяем rate limit
	if userID != 0 && !h.rateLimiter.IsAllowed(userID) {
		h.logger.Warn("rate limit exceeded", zap.Int64("user_id", userID))
		// Для обычных сообщений отправляем предупреждение
		if update.Message != nil {
			return h.sendErrorMessage(update.Message.Chat.ID, "⚠️ Слишком много запросов. Подождите минуту.")
		}
		// Для callback просто игнорируем
		return nil
	}

	// Обрабатываем inline кнопки
	if update.CallbackQuery != nil {
		return h.handleCallbackQuery(ctx, update.CallbackQuery)
	}

	// Логируем входящее сообщение
	h.logger.Debug("получено обновление",
		zap.Int64("chat_id", update.Message.Chat.ID),
		zap.String("text", update.Message.Text),
		zap.String("username", update.Message.From.UserName))

	// Записываем метрику активности пользователя
	h.userMetrics.RecordUserLogin(update.Message.From.ID)

	// Получаем или создаем пользователя с валидацией
	user, err := h.userService.GetOrCreateUser(
		ctx,
		update.Message.From.ID,
		h.sanitizeUsername(update.Message.From.UserName),
		h.sanitizeText(update.Message.From.FirstName),
		h.sanitizeText(update.Message.From.LastName),
	)
	if err != nil {
		h.logger.Error("ошибка получения пользователя", zap.Error(err))
		return h.sendErrorMessage(update.Message.Chat.ID, "Ошибка обработки запроса")
	}

	// Обрабатываем команды
	if update.Message.IsCommand() {
		return h.handleCommand(ctx, update.Message, user)
	}

	// Обрабатываем аудио сообщения
	if update.Message.Voice != nil || update.Message.Audio != nil {
		return h.handleAudioMessage(ctx, update.Message, user)
	}

	// Обрабатываем кнопки и обычные сообщения
	return h.handleButtonPress(ctx, update.Message, user)
}

// handleCommand обрабатывает команды
func (h *Handler) handleCommand(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	switch message.Command() {
	case "start":
		return h.handleStartCommand(ctx, message, user)
	case "help":
		return h.handleHelpCommand(ctx, message, user)
	case "stats":
		return h.handleStatsCommand(ctx, message, user)

	case "clear":
		return h.handleClearCommand(ctx, message, user)
	case "premium":
		return h.handlePremiumCommand(ctx, message, user)
	case "flashcards":
		return h.flashcardHandler.HandleFlashcardsCommand(ctx, message.Chat.ID, user.ID, user.Level)
	case "learning":
		return h.handleLearningCommand(ctx, message, user)

	default:
		return h.sendMessage(message.Chat.ID, h.messages.UnknownCommand())
	}
}

// generateSecureFileName генерирует безопасное имя файла
func (h *Handler) generateSecureFileName(extension string) (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("ошибка генерации случайного имени: %w", err)
	}

	// Очищаем расширение от потенциально опасных символов
	cleanExt := filepath.Ext(filepath.Base(extension))
	if cleanExt == "" {
		cleanExt = ".tmp"
	}

	return hex.EncodeToString(bytes) + cleanExt, nil
}

// sanitizeText очищает текст от потенциально опасного содержимого
func (h *Handler) sanitizeText(text string) string {
	// Ограничиваем длину
	if len(text) > MaxTextLength {
		text = text[:MaxTextLength]
	}

	// Проверяем валидность UTF-8
	if !utf8.ValidString(text) {
		text = string([]rune(text)) // Преобразуем в валидный UTF-8
	}

	// Убираем потенциально опасные символы
	text = strings.ReplaceAll(text, "\x00", "") // Null bytes
	text = strings.ReplaceAll(text, "\r", "")   // Carriage returns

	return strings.TrimSpace(text)
}

// sanitizeUsername очищает username от опасных символов
func (h *Handler) sanitizeUsername(username string) string {
	if len(username) > MaxUsernameLength {
		username = username[:MaxUsernameLength]
	}

	// Разрешаем только безопасные символы
	reg := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	return reg.ReplaceAllString(username, "")
}

// validateFileSize проверяет размер файла
func (h *Handler) validateFileSize(size int) bool {
	return size > 0 && size <= MaxFileSize
}

// handleCallbackQuery обрабатывает inline кнопки
func (h *Handler) handleCallbackQuery(ctx context.Context, callback *tgbotapi.CallbackQuery) error {
	// Получаем пользователя с валидацией
	user, err := h.userService.GetOrCreateUser(
		ctx,
		callback.From.ID,
		h.sanitizeUsername(callback.From.UserName),
		h.sanitizeText(callback.From.FirstName),
		h.sanitizeText(callback.From.LastName),
	)
	if err != nil {
		h.logger.Error("ошибка получения пользователя для callback", zap.Error(err))
		return err
	}

	// Отвечаем на callback (убираем "загрузку" кнопки)
	callbackConfig := tgbotapi.NewCallback(callback.ID, "")
	if _, err := h.bot.Request(callbackConfig); err != nil {
		h.logger.Error("ошибка ответа на callback", zap.Error(err))
	}

	data := callback.Data
	h.logger.Info("обрабатываем callback", zap.String("data", data), zap.Int64("user_id", user.ID), zap.String("user_state", user.CurrentState))
	switch {
	case strings.HasPrefix(data, "premium_plan_"):
		// Обрабатываем выбор плана премиума
		planIDStr := strings.TrimPrefix(data, "premium_plan_")
		planID, err := strconv.Atoi(planIDStr)
		if err != nil {
			h.logger.Error("ошибка парсинга ID плана", zap.Error(err))
			return err
		}

		h.logger.Info("🔍 Вызываем handlePremiumPlanSelection",
			zap.String("data", data),
			zap.Int("plan_id", planID),
			zap.Int64("user_id", user.ID))

		return h.handlePremiumPlanSelection(ctx, callback.Message.Chat.ID, user.ID, planID)

	case data == "premium_stats":
		// Показываем статистику премиума
		return h.handlePremiumCommand(ctx, callback.Message, user)

	// Обработка карточек
	case strings.HasPrefix(data, "flashcard_") || data == "flashcard_show_translation":
		return h.flashcardHandler.HandleFlashcardCallback(ctx, callback, user.ID, user.Level)

	case strings.HasPrefix(data, "test_answer_"):
		// Обрабатываем ответ на вопрос теста
		h.logger.Info("получен ответ на тест", zap.String("data", data), zap.Int64("user_id", user.ID))
		answerStr := strings.TrimPrefix(data, "test_answer_")
		answer, err := strconv.Atoi(answerStr)
		if err != nil {
			h.logger.Error("ошибка парсинга ответа теста", zap.Error(err))
			return err
		}
		return h.handleLevelTestCallback(ctx, callback, user, answer)

	case data == "test_cancel":
		// Отменяем тест
		return h.handleTestCancelCallback(ctx, callback, user)

	case strings.HasPrefix(data, "level_change_"):
		// Меняем уровень пользователя
		newLevel := strings.TrimPrefix(data, "level_change_")
		return h.handleLevelChangeCallback(ctx, callback, user, newLevel)

	case data == "level_keep_current":
		// Оставляем текущий уровень
		return h.handleKeepCurrentLevelCallback(ctx, callback, user)

	// Обработка главного меню
	case data == "main_help":
		return h.handleMainHelpCallback(ctx, callback, user)

	case data == "main_premium":
		return h.handleMainPremiumCallback(ctx, callback, user)

	case data == "main_rating":
		return h.handleMainRatingCallback(ctx, callback, user)

	case data == "learning_menu":
		return h.handleLearningMenuCallback(ctx, callback, user)

	case data == "main_stats":
		return h.handleMainStatsCallback(ctx, callback, user)

	case strings.HasPrefix(data, "tts_"):
		// Обрабатываем TTS callback
		encodedText := strings.TrimPrefix(data, "tts_")
		textBytes, err := base64.StdEncoding.DecodeString(encodedText)
		if err != nil {
			h.logger.Error("ошибка декодирования TTS текста", zap.Error(err))
			msg := tgbotapi.NewCallback(callback.ID, "❌ Ошибка обработки текста")
			h.bot.Request(msg)
			return err
		}
		text := string(textBytes)
		return h.handleTTSCallback(ctx, callback, user, text)

	default:
		h.logger.Warn("неизвестный callback", zap.String("data", data))
		return nil
	}
}

// handlePremiumPlanSelection обрабатывает выбор плана премиума
func (h *Handler) handlePremiumPlanSelection(ctx context.Context, chatID int64, userID int64, planID int) error {
	h.logger.Info("🚀 handlePremiumPlanSelection вызван",
		zap.Int64("chat_id", chatID),
		zap.Int64("user_id", userID),
		zap.Int("plan_id", planID))

	// Получаем план
	plans := h.premiumService.GetPremiumPlans()
	var selectedPlan models.PremiumPlan
	for _, plan := range plans {
		if plan.ID == planID {
			selectedPlan = plan
			break
		}
	}

	if selectedPlan.ID == 0 {
		return h.sendMessage(chatID, "План не найден")
	}

	// Создаем платеж через YooKassa API
	_, paymentID, confirmationURL, err := h.premiumService.CreatePayment(ctx, userID, planID)
	if err != nil {
		h.logger.Error("ошибка создания платежа", zap.Error(err))
		return h.sendMessage(chatID, "Ошибка создания платежа. Попробуйте позже.")
	}

	h.logger.Info("💳 Платеж создан через YooKassa",
		zap.String("payment_id", paymentID),
		zap.String("confirmation_url", confirmationURL),
		zap.Int64("user_id", userID),
		zap.Int("plan_id", planID))

	// Проверяем, что ссылка не пустая
	if confirmationURL == "" {
		h.logger.Error("пустая ссылка на оплату",
			zap.String("payment_id", paymentID),
			zap.Int64("user_id", userID))
		return h.sendMessage(chatID, "Ошибка генерации ссылки на оплату. Попробуйте позже.")
	}

	// Отправляем ссылку на оплату
	messageText := fmt.Sprintf(`💳 <b>Платеж создан!</b>

📋 <b>План:</b> %s
💰 <b>Сумма:</b> %.0f %s
⏱ <b>Длительность:</b> %d дней

🔗 <b>Ссылка для оплаты:</b>
<a href="%s">Оплатить %.0f %s</a>

💳 <b>Доступные способы оплаты:</b>
• Банковские карты (Visa, MasterCard, МИР)
• СБП (Система быстрых платежей)
• Электронные кошельки
• QR-код для мобильных приложений

⚠️ <i>После оплаты премиум-подписка будет активирована автоматически</i>`,
		selectedPlan.Name, selectedPlan.Price, selectedPlan.Currency,
		selectedPlan.DurationDays, confirmationURL, selectedPlan.Price, selectedPlan.Currency)

	msg := tgbotapi.NewMessage(chatID, messageText)
	msg.ParseMode = "HTML"

	_, err = h.bot.Send(msg)
	return err
}

// handleButtonPress обрабатывает нажатия кнопок
func (h *Handler) handleButtonPress(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	text := message.Text

	switch text {
	case "📊 Статистика":
		return h.handleStatsCommand(ctx, message, user)
	case "❓ Помощь":
		return h.handleHelpCommand(ctx, message, user)
	case "🗑 Очистить диалог":
		return h.handleClearCommand(ctx, message, user)
	case "🔙 Назад к меню":
		// Если пользователь в тесте, завершаем его без результатов
		if user.CurrentState == models.StateInLevelTest {
			return h.cancelLevelTest(ctx, message, user)
		}
		return h.handleStartCommand(ctx, message, user)
	case "🎯 Тест уровня":
		return h.handleLevelTestButton(ctx, message, user)
	case "🎓 Тест уровня":
		return h.handleLevelTestButton(ctx, message, user)
	case "🏆 Рейтинг":
		return h.handleLeaderboardButton(ctx, message, user)
	case "🎯 Начать тест":
		return h.handleStartLevelTest(ctx, message, user)
	case "💎 Премиум":
		return h.handlePremiumCommand(ctx, message, user)
	case "📚 Обучение":
		return h.handleLearningButton(ctx, message, user)
	case "🔗 Реферальная ссылка":
		return h.handleReferralButton(ctx, message, user)
	case "📝 Словарные карточки":
		return h.flashcardHandler.HandleFlashcardsCommand(ctx, message.Chat.ID, user.ID, user.Level)
	case "🔙 Назад в главное меню":
		return h.handleStartCommand(ctx, message, user)
	default:
		// Если это не кнопка, а обычное сообщение
		if message.Text != "" {
			return h.handleMessage(ctx, message, user)
		}
		// Игнорируем неизвестные кнопки
		return nil
	}
}

// addXP добавляет опыт пользователю
func (h *Handler) addXP(user *models.User, xp int) {
	oldLevel := user.Level
	oldXP := user.XP

	user.XP += xp

	// Определяем новый уровень на основе XP
	newLevel := models.GetLevelByXP(user.XP)

	// Проверяем, повысился ли уровень
	if oldLevel != newLevel {
		user.Level = newLevel

		// Отправляем уведомление о повышении уровня
		go h.sendLevelUpNotification(user.ID, oldLevel, newLevel, user.XP)

		h.logger.Info("пользователь повысил уровень",
			zap.Int64("user_id", user.ID),
			zap.String("old_level", oldLevel),
			zap.String("new_level", newLevel),
			zap.Int("total_xp", user.XP))
	}

	// Обновляем пользователя в базе данных
	updateReq := &models.UpdateUserRequest{
		XP:    &user.XP,
		Level: &user.Level,
	}

	ctx := context.Background()
	_, err := h.userService.UpdateUser(ctx, user.ID, updateReq)
	if err != nil {
		h.logger.Error("ошибка обновления XP пользователя",
			zap.Error(err),
			zap.Int64("user_id", user.ID),
			zap.Int("old_xp", oldXP),
			zap.Int("new_xp", user.XP))
	}
}

// updateUserDataFromDB обновляет данные пользователя из базы данных
func (h *Handler) updateUserDataFromDB(ctx context.Context, user *models.User) {
	updatedUser, err := h.userService.GetUserByID(ctx, user.ID)
	if err == nil && updatedUser != nil {
		user.IsPremium = updatedUser.IsPremium
		user.PremiumExpiresAt = updatedUser.PremiumExpiresAt
		user.MessagesCount = updatedUser.MessagesCount
		user.MaxMessages = updatedUser.MaxMessages
	}
}

// handleMessageLimit показывает сообщение о лимите сообщений
func (h *Handler) handleMessageLimit(ctx context.Context, chatID int64, user *models.User) error {
	// Показываем сообщение о лимите и предлагаем премиум
	stats, _ := h.premiumService.GetUserStats(ctx, user.ID)

	// Обновляем данные пользователя в памяти после проверки статуса
	h.updateUserDataFromDB(ctx, user)

	limitMessage := fmt.Sprintf(`🚫 <b>Достигнут лимит сообщений!</b>

📊 Ваша статистика:
• Отправлено сообщений: %d
• Лимит на сегодня: %d

💎 <b>Обновитесь до премиума</b> для безлимитного общения!

Используйте команду /premium для покупки подписки.`,
		stats["messages_count"], stats["max_messages"])

	return h.sendMessage(chatID, limitMessage)
}

// updateStudyActivity обновляет активность обучения
func (h *Handler) updateStudyActivity(user *models.User) {
	// Проверяем, нужно ли обновлять study streak
	now := time.Now()
	shouldUpdate := false

	if user.LastStudyDate.IsZero() {
		// Первое изучение
		shouldUpdate = true
	} else {
		// Проверяем, занимается ли пользователь в тот же день
		lastStudyDate := user.LastStudyDate.Truncate(24 * time.Hour)
		currentDate := now.Truncate(24 * time.Hour)

		if lastStudyDate.Before(currentDate) {
			// Пользователь занимается в новый день
			shouldUpdate = true
		}
		// Если lastStudyDate == currentDate, то пользователь уже занимался сегодня
	}

	if !shouldUpdate {
		// Не обновляем, если пользователь уже занимался сегодня
		return
	}

	err := h.userService.UpdateStudyActivity(context.Background(), user.ID)
	if err != nil {
		h.logger.Error("ошибка обновления активности обучения", zap.Error(err))
		return
	}

	// Обновляем данные пользователя в памяти
	updatedUser, err := h.userService.GetUserByID(context.Background(), user.ID)
	if err != nil {
		h.logger.Error("ошибка получения обновленного пользователя", zap.Error(err))
		return
	}

	if updatedUser != nil {
		user.StudyStreak = updatedUser.StudyStreak
		user.LastStudyDate = updatedUser.LastStudyDate

		// Записываем метрику study streak
		// h.userMetrics.RecordStudyStreak(user.ID, updatedUser.StudyStreak) // TODO: добавить метрику
	}
}

// handleMessage обрабатывает обычные сообщения
func (h *Handler) handleMessage(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	// Проверяем, находится ли пользователь в тесте уровня
	if user.CurrentState == models.StateInLevelTest {
		// Проверяем, не хочет ли пользователь отменить тест
		if message.Text == "❌ Отменить тест" {
			return h.cancelLevelTest(ctx, message, user)
		}

		return h.handleLevelTestAnswer(ctx, message, user)
	}

	// Активируем реферал если пользователь был приглашен и отправляет первое сообщение
	if user.ReferredBy != nil {
		err := h.referralService.ActivateReferral(ctx, user.ID)
		if err != nil {
			h.logger.Error("ошибка активации реферала",
				zap.Error(err),
				zap.Int64("user_id", user.ID),
				zap.Int64("referred_by", *user.ReferredBy))
			// Не возвращаем ошибку, продолжаем обработку сообщения
		} else {
			h.logger.Info("реферал активирован",
				zap.Int64("user_id", user.ID),
				zap.Int64("referred_by", *user.ReferredBy))
		}
	}

	// Записываем метрику сообщения пользователя
	h.userMetrics.RecordUserMessage("text")

	// Сохраняем сообщение пользователя с санитизацией
	sanitizedText := h.sanitizeText(message.Text)
	_, err := h.messageService.SaveUserMessage(ctx, user.ID, sanitizedText)
	if err != nil {
		h.logger.Error("ошибка сохранения сообщения пользователя", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка сохранения сообщения")
	}

	// Проверяем, на английском ли сообщение
	if h.isEnglishMessage(message.Text) {
		return h.handleEnglishMessage(ctx, message, user)
	}

	// Если сообщение на русском, переводим в режим общения
	return h.handleRussianMessage(ctx, message, user)
}

// isEnglishMessage проверяет, написано ли сообщение на английском
func (h *Handler) isEnglishMessage(text string) bool {
	// Простая проверка: если в тексте больше латинских букв, чем кириллических
	englishChars := 0
	russianChars := 0

	for _, char := range strings.ToLower(text) {
		if char >= 'a' && char <= 'z' {
			englishChars++
		} else if (char >= 'а' && char <= 'я') || char == 'ё' {
			russianChars++
		}
	}

	result := englishChars > russianChars && englishChars > 0
	h.logger.Info("🔍 isEnglishMessage", zap.String("text", text), zap.Int("english_chars", englishChars), zap.Int("russian_chars", russianChars), zap.Bool("is_english", result))
	return result
}

// handleEnglishMessage обрабатывает сообщения на английском языке
func (h *Handler) handleEnglishMessage(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	h.logger.Info("🔍 handleEnglishMessage вызван", zap.String("text", message.Text))

	// Проверяем лимит сообщений для бесплатных пользователей
	canSend, err := h.premiumService.CanSendMessage(ctx, user.ID)
	if err != nil {
		h.logger.Error("ошибка проверки лимита сообщений", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка проверки лимита сообщений")
	}

	if !canSend {
		return h.handleMessageLimit(ctx, message.Chat.ID, user)
	}

	// Получаем историю диалога для контекста (пока не используется)
	_, err = h.messageService.GetChatHistory(ctx, user.ID, ChatHistoryForConversation)
	if err != nil {
		h.logger.Error("ошибка получения истории диалога", zap.Error(err))
		// Продолжаем без контекста
	}

	// Получаем или создаем контекст диалога
	dialogContext := h.getOrCreateDialogContext(user.ID, user.Level)

	// Добавляем сообщение пользователя в контекст
	dialogContext.AddUserMessage(message.Text)

	// Создаем AI сообщения с контекстом диалога
	var aiMessages []ai.Message

	// Системный промпт для английских сообщений (отправляется только один раз)
	aiMessages = append(aiMessages, ai.Message{
		Role:    "system",
		Content: h.prompts.GetEnglishMessagePrompt(user.Level),
	})

	// Добавляем текущее сообщение пользователя
	aiMessages = append(aiMessages, ai.Message{
		Role:    "user",
		Content: message.Text,
	})

	start := time.Now()
	options := ai.GenerationOptions{
		Temperature: 0.7,
		MaxTokens:   500,
	}
	response, err := h.aiClient.GenerateResponse(ctx, aiMessages, options)
	duration := time.Since(start)

	h.aiMetrics.RecordAIRequest("english_with_translation", err == nil, duration.Seconds())

	if err != nil {
		h.logger.Error("ошибка генерации ответа с переводом", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Произошла ошибка при генерации ответа")
	}

	// Сохраняем ответ ассистента (только английская часть, без перевода)
	_, err = h.messageService.SaveAssistantMessage(ctx, user.ID, response.Content)
	if err != nil {
		h.logger.Error("ошибка сохранения ответа", zap.Error(err))
	}

	// Добавляем ответ ассистента в контекст диалога
	dialogContext.AddAssistantMessage(response.Content)

	// Увеличиваем счетчик сообщений пользователя
	if err := h.premiumService.IncrementMessageCount(ctx, user.ID); err != nil {
		h.logger.Error("ошибка увеличения счетчика сообщений", zap.Error(err))
	}

	// Даем XP за любое общение на английском
	xp := 15 // Все получают максимум - главное общение

	// Добавляем XP и обновляем активность
	h.addXP(user, xp)
	h.updateStudyActivity(user) // Обновляем study streak только раз в день
	h.userMetrics.RecordXP(user.ID, xp, "english_message")

	return h.sendMessageWithTTS(message.Chat.ID, h.cleanAIResponse(response.Content))
}

// handleRussianMessage обрабатывает сообщения на русском языке
func (h *Handler) handleRussianMessage(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	// Проверяем, просит ли пользователь перевод
	lowerText := strings.ToLower(message.Text)
	// Проверяем, просит ли пользователь задание
	if strings.Contains(lowerText, "задание") ||
		strings.Contains(lowerText, "упражнение") ||
		strings.Contains(lowerText, "урок") ||
		strings.Contains(lowerText, "дай мне") ||
		strings.Contains(lowerText, "exercise") {
		return h.handleExerciseRequest(ctx, message, user)
	}

	// Проверяем лимит сообщений для бесплатных пользователей
	canSend, err := h.premiumService.CanSendMessage(ctx, user.ID)
	if err != nil {
		h.logger.Error("ошибка проверки лимита сообщений", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка проверки лимита сообщений")
	}

	if !canSend {
		return h.handleMessageLimit(ctx, message.Chat.ID, user)
	}

	// Получаем историю диалога для контекста (пока не используется)
	_, err = h.messageService.GetChatHistory(ctx, user.ID, ChatHistoryForConversation)
	if err != nil {
		h.logger.Error("ошибка получения истории диалога", zap.Error(err))
		// Продолжаем без контекста
	}

	// Получаем или создаем контекст диалога
	dialogContext := h.getOrCreateDialogContext(user.ID, user.Level)

	// Добавляем сообщение пользователя в контекст
	dialogContext.AddUserMessage(message.Text)

	// Получаем историю диалога для контекста
	history, err := h.messageService.GetChatHistory(ctx, user.ID, 10) // Последние 10 сообщений
	if err != nil {
		h.logger.Error("ошибка получения истории диалога", zap.Error(err))
		// Продолжаем без контекста
	}

	// Создаем AI сообщения с контекстом диалога
	var aiMessages []ai.Message

	// Системный промпт для русских сообщений
	aiMessages = append(aiMessages, ai.Message{
		Role:    "system",
		Content: h.prompts.GetRussianMessagePrompt(user.Level),
	})

	// Добавляем историю диалога для контекста
	if history != nil && len(history.Messages) > 1 {
		// Берем последние 8 сообщений (исключая текущее)
		start := 0
		if len(history.Messages) > 8 {
			start = len(history.Messages) - 8
		}

		for i := start; i < len(history.Messages)-1; i++ { // -1 чтобы исключить текущее сообщение
			msg := history.Messages[i]
			aiMessages = append(aiMessages, ai.Message{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// Добавляем текущее сообщение пользователя
	aiMessages = append(aiMessages, ai.Message{
		Role:    "user",
		Content: message.Text,
	})

	start := time.Now()
	options := ai.GenerationOptions{
		Temperature: 0.7,
		MaxTokens:   500,
	}
	response, err := h.aiClient.GenerateResponse(ctx, aiMessages, options)
	duration := time.Since(start)

	h.aiMetrics.RecordAIRequest("russian_with_translation", err == nil, duration.Seconds())

	if err != nil {
		h.logger.Error("ошибка генерации ответа с переводом", zap.Error(err))
		return h.sendMessage(message.Chat.ID, "Let's try chatting in English! 🇬🇧\n\n<tg-spoiler>🇷🇺 Давай попробуем общаться на английском!</tg-spoiler>")
	}

	// Извлекаем только английскую часть для сохранения в БД
	englishOnly := h.extractEnglishFromResponse(response.Content)

	// Сохраняем ответ ассистента (только английская часть)
	_, err = h.messageService.SaveAssistantMessage(ctx, user.ID, englishOnly)
	if err != nil {
		h.logger.Error("ошибка сохранения ответа", zap.Error(err))
	}

	// Добавляем ответ ассистента в контекст диалога
	dialogContext.AddAssistantMessage(response.Content)

	// Увеличиваем счетчик сообщений пользователя
	if err := h.premiumService.IncrementMessageCount(ctx, user.ID); err != nil {
		h.logger.Error("ошибка увеличения счетчика сообщений", zap.Error(err))
	}

	// Небольшой XP за участие
	h.addXP(user, 3)
	h.updateStudyActivity(user) // Обновляем study streak только раз в день
	h.userMetrics.RecordXP(user.ID, 3, "russian_message")

	return h.sendMessage(message.Chat.ID, response.Content)
}

// handleExerciseRequest обрабатывает запросы на упражнения/задания
func (h *Handler) handleExerciseRequest(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	// Получаем историю последних упражнений для избежания дублирования
	recentHistory, err := h.messageService.GetChatHistory(ctx, user.ID, 5)
	if err != nil {
		h.logger.Error("ошибка получения истории для упражнений", zap.Error(err))
		// Продолжаем без истории
	}

	// Генерируем быстрое упражнение в зависимости от уровня с учетом истории
	exercisePrompt := h.prompts.GetExercisePromptWithHistory(user.Level, recentHistory)

	aiMessages := []ai.Message{
		{Role: "user", Content: exercisePrompt},
	}

	start := time.Now()
	options := ai.GenerationOptions{
		Temperature: 1.2, // Увеличиваем температуру для большей случайности
		MaxTokens:   300,
	}
	response, err := h.aiClient.GenerateResponse(ctx, aiMessages, options)
	duration := time.Since(start)

	h.aiMetrics.RecordAIRequest("exercise_generation", err == nil, duration.Seconds())

	if err != nil {
		h.logger.Error("ошибка генерации упражнения", zap.Error(err))
		return h.sendMessage(message.Chat.ID, fmt.Sprintf(`Exercise: Choose the correct form of the verb
Question: She _____ to work every day.
Options: go/goes/going

<tg-spoiler>🇷🇺 Выбери правильную форму глагола: Она ... на работу каждый день</tg-spoiler>

*Уровень: %s*`, h.getLevelText(user.Level)))
	}

	// Извлекаем только английскую часть для сохранения в БД
	englishOnly := h.extractEnglishFromResponse(response.Content)

	// Сохраняем ответ ассистента
	_, err = h.messageService.SaveAssistantMessage(ctx, user.ID, englishOnly)
	if err != nil {
		h.logger.Error("ошибка сохранения упражнения", zap.Error(err))
	}

	// Даем XP за запрос упражнения
	h.addXP(user, 5)
	h.updateStudyActivity(user) // Обновляем study streak только раз в день
	h.userMetrics.RecordXP(user.ID, 5, "exercise_request")

	return h.sendMessage(message.Chat.ID, response.Content)
}

// handleStartCommand обрабатывает команду /start
func (h *Handler) handleStartCommand(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	// Обновляем study streak только раз в день
	h.updateStudyActivity(user)

	// Проверяем реферальные параметры
	if message.CommandArguments() != "" {
		args := message.CommandArguments()
		if strings.HasPrefix(args, "ref_") {
			referralCode := strings.TrimPrefix(args, "ref_")

			// Находим пользователя по реферальному коду
			referrer, err := h.referralService.ValidateReferralCode(ctx, referralCode)
			if err != nil {
				h.logger.Error("неверный реферальный код",
					zap.Error(err),
					zap.String("referral_code", referralCode))
				// Не показываем ошибку пользователю, просто продолжаем
			} else {
				// Создаем реферальную связь
				err = h.referralService.CreateReferral(ctx, referrer.ID, user.ID)
				if err != nil {
					h.logger.Error("ошибка создания реферальной связи",
						zap.Error(err),
						zap.String("referral_code", referralCode),
						zap.Int64("referrer_id", referrer.ID),
						zap.Int64("referred_id", user.ID))
					// Не показываем ошибку пользователю, просто продолжаем
				} else {
					h.logger.Info("реферальная связь создана",
						zap.String("referral_code", referralCode),
						zap.Int64("referrer_id", referrer.ID),
						zap.Int64("referred_id", user.ID))
				}
			}
		}
	}

	welcomeText := h.messages.Welcome(user.FirstName, h.getLevelText(user.Level), user.XP)
	return h.sendMessageWithKeyboard(message.Chat.ID, welcomeText, h.messages.GetMainKeyboard())
}

// handleHelpCommand обрабатывает команду /help
func (h *Handler) handleHelpCommand(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	// Обновляем study streak только раз в день
	h.updateStudyActivity(user)

	return h.sendMessage(message.Chat.ID, h.messages.Help())
}

// handleStatsCommand обрабатывает команду /stats
func (h *Handler) handleStatsCommand(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	// Обновляем study streak только раз в день
	h.updateStudyActivity(user)

	stats, err := h.userService.GetUserStats(ctx, user.ID)
	if err != nil {
		h.logger.Error("ошибка получения статистики", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка получения статистики")
	}

	statsText := h.messages.Stats(
		user.FirstName,
		h.getLevelText(user.Level),
		user.XP,
		stats.StudyStreak,
		stats.LastStudyDate.Format(time.DateTime),
	)

	return h.sendMessage(message.Chat.ID, statsText)
}

// handleClearCommand обрабатывает команду /clear
func (h *Handler) handleClearCommand(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	// Очищаем историю диалога
	err := h.messageService.ClearChatHistory(ctx, user.ID)
	if err != nil {
		h.logger.Error("ошибка очистки истории диалога", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка очистки истории")
	}

	// Сбрасываем состояние пользователя
	user.CurrentState = models.StateIdle

	// Удаляем активный тест уровня, если есть
	delete(h.activeLevelTests, user.ID)

	// Обновляем пользователя в базе данных
	currentState := models.StateIdle
	updateReq := &models.UpdateUserRequest{
		CurrentState: &currentState,
	}
	_, err = h.userService.UpdateUser(ctx, user.ID, updateReq)
	if err != nil {
		h.logger.Error("ошибка сброса состояния пользователя", zap.Error(err))
	}

	return h.sendMessageWithKeyboard(message.Chat.ID,
		h.messages.ChatCleared(),
		h.messages.GetMainKeyboard())
}

// handlePremiumCommand обрабатывает команду премиум-подписки
func (h *Handler) handlePremiumCommand(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	// Получаем статистику пользователя
	stats, err := h.premiumService.GetUserStats(ctx, user.ID)
	if err != nil {
		h.logger.Error("ошибка получения статистики премиума", zap.Error(err))
		return h.sendMessage(message.Chat.ID, "Ошибка получения статистики")
	}

	// Создаем клавиатуру с планами премиума
	plans := h.premiumService.GetPremiumPlans()
	var keyboard [][]tgbotapi.InlineKeyboardButton

	for _, plan := range plans {
		button := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("💶 %s - %.0f %s", plan.Name, plan.Price, plan.Currency),
			fmt.Sprintf("premium_plan_%d", plan.ID),
		)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{button})
	}

	// Кнопка статистики убрана - вся информация уже показана в сообщении выше
	// Для бесплатных пользователей статистика показана в тексте сообщения
	// Для премиум пользователей статистика тоже показана в тексте сообщения

	inlineKeyboard := tgbotapi.NewInlineKeyboardMarkup(keyboard...)

	// Формируем сообщение
	var messageText string
	if stats["is_premium"].(bool) {
		var expiresAt string
		if stats["premium_expires_at"] != nil {
			expiresAt = stats["premium_expires_at"].(string)
		} else {
			expiresAt = "неизвестно"
		}

		messageText = fmt.Sprintf(`🌟 <b>Премиум-подписка активна!</b>

✅ Ваши преимущества:
• Безлимитные сообщения
• Приоритетная поддержка
• Расширенные упражнения
• Персональные рекомендации

📅 Действует до: %s

Вы можете продлить подписку, выбрав один из планов ниже:`, expiresAt)
	} else {
		remaining := stats["remaining_messages"]
		messageText = fmt.Sprintf(`💎 <b>Бесплатная подписка</b>

📊 Ваша статистика:
• Отправлено сообщений: %d
• Осталось сообщений: %v
• Лимит на сегодня: %d

🚀 <b>Преимущества премиума:</b>
• Безлимитные сообщения
• Приоритетная поддержка
• Расширенные упражнения
• Персональные рекомендации

Выберите план подписки:`,
			stats["messages_count"], remaining, stats["max_messages"])
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, messageText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = inlineKeyboard

	_, err = h.bot.Send(msg)
	return err
}

// handleLevelTestButton обрабатывает нажатие кнопки "Тест уровня"
func (h *Handler) handleLevelTestButton(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	// Проверяем, не проходит ли пользователь уже тест
	if user.CurrentState == models.StateInLevelTest {
		return h.sendMessage(message.Chat.ID, "Вы уже проходите тест уровня. Завершите текущий тест или используйте /clear для сброса.")
	}

	// Показываем введение к тесту
	return h.sendMessageWithKeyboard(message.Chat.ID,
		h.messages.LevelTestIntro(),
		h.messages.GetLevelTestKeyboard())
}

// handleStartLevelTest обрабатывает начало теста уровня
func (h *Handler) handleStartLevelTest(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	// Проверяем, проходил ли пользователь тест сегодня
	today := time.Now().Format("2006-01-02")
	if user.LastTestDate != nil && user.LastTestDate.Format("2006-01-02") == today {
		return h.sendMessage(message.Chat.ID, `❌ <b>Тест уже пройден сегодня!</b>

🕐 <b>Ограничение:</b> Тест уровня можно проходить только <b>один раз в день</b>

⏰ <b>Следующий тест:</b> завтра
💡 <b>Совет:</b> Используй это время для изучения английского!

🎯 <b>Команды для практики:</b>
• Напиши мне на английском
• Попроси <b>"дай задание"</b> для упражнений
• Используй <b>/stats</b> для просмотра прогресса`)
	}

	// Создаем новый тест
	levelTest := h.generateLevelTest(user.ID)
	h.activeLevelTests[user.ID] = levelTest

	// Обновляем состояние пользователя
	newState := models.StateInLevelTest
	updateReq := &models.UpdateUserRequest{
		CurrentState: &newState,
	}
	_, err := h.userService.UpdateUser(ctx, user.ID, updateReq)
	if err != nil {
		h.logger.Error("ошибка обновления состояния пользователя", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка запуска теста")
	}

	user.CurrentState = models.StateInLevelTest

	// Показываем первый вопрос
	return h.showCurrentQuestion(ctx, message.Chat.ID, user)
}

// showCurrentQuestion показывает текущий вопрос теста
func (h *Handler) showCurrentQuestion(ctx context.Context, chatID int64, user *models.User) error {
	levelTest, exists := h.activeLevelTests[user.ID]
	if !exists {
		return h.sendErrorMessage(chatID, "Тест не найден. Начните новый тест.")
	}

	if levelTest.CurrentQuestion >= len(levelTest.Questions) {
		// Тест завершен
		return h.completeLevelTest(ctx, chatID, user)
	}

	currentQ := levelTest.Questions[levelTest.CurrentQuestion]

	// Формируем текст вопроса с вариантами ответов
	questionText := fmt.Sprintf(`🎯 <b>Вопрос %d из %d</b>

%s

<b>Варианты ответов:</b>`,
		levelTest.CurrentQuestion+1,
		len(levelTest.Questions),
		currentQ.Question)

	// Добавляем варианты ответов в текст
	for i, option := range currentQ.Options {
		questionText += fmt.Sprintf("\n%d. %s", i+1, option)
	}

	questionText += "\n\n💡 <b>Выберите правильный ответ:</b>"

	// Создаем inline-клавиатуру с вариантами ответов
	keyboard := tgbotapi.NewInlineKeyboardMarkup(h.messages.GetTestAnswerKeyboard(currentQ.Options)...)

	h.logger.Info("отправляем вопрос с inline-клавиатурой",
		zap.Int("question_num", levelTest.CurrentQuestion+1),
		zap.Int("options_count", len(currentQ.Options)),
		zap.Int64("user_id", user.ID))

	msg := tgbotapi.NewMessage(chatID, questionText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err := h.bot.Send(msg)
	if err != nil {
		h.logger.Error("ошибка отправки вопроса с клавиатурой", zap.Error(err))
	}
	return err
}

// completeLevelTest завершает тест и показывает результаты
func (h *Handler) completeLevelTest(ctx context.Context, chatID int64, user *models.User) error {
	levelTest, exists := h.activeLevelTests[user.ID]
	if !exists {
		return h.sendErrorMessage(chatID, "Тест не найден.")
	}

	// Отмечаем время завершения
	now := time.Now()
	levelTest.CompletedAt = &now

	// Определяем рекомендуемый уровень на основе теста
	recommendedLevel, levelDescription := h.calculateLevel(levelTest.Score, levelTest.MaxScore)

	// Сбрасываем состояние пользователя и записываем дату прохождения теста
	newState := models.StateIdle
	updateReq := &models.UpdateUserRequest{
		CurrentState: &newState,
		LastTestDate: &now,
	}
	_, err := h.userService.UpdateUser(ctx, user.ID, updateReq)
	if err != nil {
		h.logger.Error("ошибка обновления состояния пользователя", zap.Error(err))
	}

	// Обновляем локальные данные пользователя
	user.CurrentState = models.StateIdle
	user.LastTestDate = &now

	// Добавляем XP за прохождение теста
	xp := 50 + (levelTest.Score * 5) // Больше XP за тест
	h.addXP(user, xp)
	h.userMetrics.RecordXP(user.ID, xp, "level_test_completed")

	// Обновляем локальный XP для отображения
	user.XP += xp

	correctAnswer := 0
	for _, level := range levelTest.Answers {
		if level.IsCorrect {
			correctAnswer++
		}
	}
	// Формируем сообщение с результатами
	percentage := float64(correctAnswer) / float64(len(levelTest.Questions)) * 100

	var recommendationText string

	if recommendedLevel != user.Level {
		recommendationText = fmt.Sprintf("\n\n🎯 <b>Рекомендация:</b> По результатам теста твой уровень - <b>%s</b>\n💡 Хочешь переключиться на этот уровень для более подходящих заданий?", h.getLevelText(recommendedLevel))
	} else {
		recommendationText = "\n\n✅ <b>Отлично!</b> Результаты теста соответствуют твоему текущему уровню."
	}

	resultText := fmt.Sprintf(`🎉 <b>Тест завершен!</b>

📊 <b>Твой результат:</b>
• Правильных ответов: %d из %d
• Процент: %.0f%%
• Рекомендуемый уровень: <b>%s</b>

📝 <b>%s</b>

⭐ <b>Получено XP:</b> +%d
💰 <b>Общий XP:</b> %d%s

🎯 Продолжай общаться на английском, чтобы повышать свой уровень!`,
		correctAnswer,
		len(levelTest.Questions),
		percentage,
		h.getLevelText(recommendedLevel),
		levelDescription,
		xp,
		user.XP,
		recommendationText)

	// Удаляем тест из активных
	delete(h.activeLevelTests, user.ID)

	// Если уровень отличается, показываем кнопки выбора
	if recommendedLevel != user.Level {
		return h.sendTestResultsWithLevelChoice(chatID, resultText, recommendedLevel)
	}

	return h.sendMessageWithKeyboard(chatID, resultText, h.messages.GetMainKeyboard())
}

// sendTestResultsWithLevelChoice отправляет результаты теста с кнопками выбора уровня
func (h *Handler) sendTestResultsWithLevelChoice(chatID int64, resultText, recommendedLevel string) error {
	// Создаем inline-клавиатуру с выбором уровня
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("✅ Переключить на %s", h.getLevelText(recommendedLevel)),
				fmt.Sprintf("level_change_%s", recommendedLevel),
			),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				"❌ Оставить текущий уровень",
				"level_keep_current",
			),
		),
	)

	msg := tgbotapi.NewMessage(chatID, resultText)
	msg.ParseMode = "HTML"
	msg.ReplyMarkup = keyboard

	_, err := h.bot.Send(msg)
	return err
}

// cancelLevelTest отменяет тест уровня без результатов
func (h *Handler) cancelLevelTest(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	// Проверяем, есть ли активный тест
	levelTest, exists := h.activeLevelTests[user.ID]
	if !exists {
		// Если теста нет, просто возвращаемся в главное меню
		return h.handleStartCommand(ctx, message, user)
	}

	// Отмечаем время отмены
	now := time.Now()
	levelTest.CompletedAt = &now

	// Сбрасываем состояние пользователя
	newState := models.StateIdle
	updateReq := &models.UpdateUserRequest{
		CurrentState: &newState,
	}
	_, err := h.userService.UpdateUser(ctx, user.ID, updateReq)
	if err != nil {
		h.logger.Error("ошибка обновления состояния пользователя", zap.Error(err))
	}

	// Обновляем локальные данные пользователя
	user.CurrentState = models.StateIdle

	// Удаляем тест из активных
	delete(h.activeLevelTests, user.ID)

	// Логируем отмену теста
	h.logger.Info("пользователь отменил тест уровня",
		zap.Int64("user_id", user.ID),
		zap.Int("questions_answered", levelTest.CurrentQuestion),
		zap.Int("score", levelTest.Score),
		zap.String("test_duration", time.Since(levelTest.StartedAt).String()))

	// Записываем метрику отмененного теста
	h.userMetrics.RecordXP(user.ID, 0, "level_test_cancelled")

	cancelMessage := `❌ <b>Тест отменен</b>

	Тестирование завершено без результатов.
	
	🎯 <b>Что дальше?</b>
	• Попробуй пройти тест позже  
	• Изучай английский в своём темпе  
	• Используй команду "<b>🎯 Тест уровня</b>", когда будешь готов`

	return h.sendMessageWithKeyboard(message.Chat.ID, cancelMessage, h.messages.GetMainKeyboard())
}

// handleLevelTestAnswer обрабатывает ответ на вопрос теста
func (h *Handler) handleLevelTestAnswer(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	levelTest, exists := h.activeLevelTests[user.ID]
	if !exists {
		return h.sendErrorMessage(message.Chat.ID, "Тест не найден. Начните новый тест.")
	}

	if levelTest.CurrentQuestion >= len(levelTest.Questions) {
		return h.completeLevelTest(ctx, message.Chat.ID, user)
	}

	// Парсим ответ пользователя
	answer := -1
	switch message.Text {
	case "1":
		answer = 0
	case "2":
		answer = 1
	case "3":
		answer = 2
	case "4":
		answer = 3
	default:
		return h.sendMessage(message.Chat.ID, "❌ Пожалуйста, отправьте номер ответа (1, 2, 3 или 4)")
	}

	currentQ := levelTest.Questions[levelTest.CurrentQuestion]
	isCorrect := answer == currentQ.CorrectAnswer
	points := 0
	if isCorrect {
		points = currentQ.Points
		levelTest.Score += points
	}

	// Сохраняем ответ
	levelTest.Answers = append(levelTest.Answers, models.LevelTestAnswer{
		QuestionID: currentQ.ID,
		Answer:     answer,
		IsCorrect:  isCorrect,
		Points:     points,
	})

	// Показываем результат ответа
	var feedback string
	if isCorrect {
		feedback = "✅ Правильно!"
	} else {
		correctOption := currentQ.Options[currentQ.CorrectAnswer]
		feedback = fmt.Sprintf("❌ Неправильно. Правильный ответ: %d. %s", currentQ.CorrectAnswer+1, correctOption)
	}

	// Добавляем информацию о возможности отмены
	feedback += "\n\n💡 <b>Подсказка:</b> Можешь отменить тест в любой момент"

	err := h.sendMessageWithKeyboard(message.Chat.ID, feedback, h.messages.GetActiveTestKeyboard())
	if err != nil {
		return err
	}

	// Переходим к следующему вопросу
	levelTest.CurrentQuestion++

	// Небольшая пауза перед следующим вопросом
	time.Sleep(2 * time.Second)

	return h.showCurrentQuestion(ctx, message.Chat.ID, user)
}

// sendMessage отправляет сообщение
func (h *Handler) sendMessage(chatID int64, text string) error {
	return h.sendSafeMessage(chatID, text, false)
}

// sendSafeMessage отправляет сообщение с защитой от битых HTML тегов
func (h *Handler) sendSafeMessage(chatID int64, text string, forceHTML bool) error {
	// Проверяем, содержит ли текст HTML теги
	hasHTML := strings.Contains(text, "<") && strings.Contains(text, ">")

	var cleanText string
	var parseMode string

	if hasHTML || forceHTML {
		// Если есть HTML теги, декодируем HTML-сущности и используем как HTML
		cleanText = html.UnescapeString(text)
		parseMode = "HTML"
	} else {
		// Если HTML тегов нет, все равно декодируем HTML-сущности
		cleanText = html.UnescapeString(text)
		parseMode = ""
	}

	msg := tgbotapi.NewMessage(chatID, cleanText)
	if parseMode != "" {
		msg.ParseMode = parseMode
	}

	_, err := h.bot.Send(msg)
	if err != nil {
		h.logger.Error("ошибка отправки сообщения",
			zap.Int64("chat_id", chatID),
			zap.String("parse_mode", parseMode),
			zap.Error(err))

		// Если HTML парсинг не удался, пробуем отправить как обычный текст
		if parseMode == "HTML" {
			h.logger.Info("повторная отправка как обычный текст", zap.Int64("chat_id", chatID))
			// Удаляем HTML теги для fallback
			fallbackText := h.stripHTMLTags(text)
			fallbackMsg := tgbotapi.NewMessage(chatID, fallbackText)
			_, fallbackErr := h.bot.Send(fallbackMsg)
			return fallbackErr
		}
		return err
	}

	return nil
}

// sendMessageWithKeyboard отправляет сообщение с клавиатурой
func (h *Handler) sendMessageWithKeyboard(chatID int64, text string, keyboard [][]string) error {
	// Проверяем, содержит ли текст HTML теги
	hasHTML := strings.Contains(text, "<") && strings.Contains(text, ">")

	var msg tgbotapi.MessageConfig
	if hasHTML {
		// Если есть HTML теги, очищаем их безопасно
		cleanText := h.cleanTextForTelegram(text)
		msg = tgbotapi.NewMessage(chatID, cleanText)
		msg.ParseMode = "HTML"
	} else {
		// Если HTML тегов нет, экранируем опасные символы
		safeText := html.EscapeString(text)
		msg = tgbotapi.NewMessage(chatID, safeText)
	}

	// Создаем клавиатуру
	var buttons [][]tgbotapi.KeyboardButton
	for _, row := range keyboard {
		var buttonRow []tgbotapi.KeyboardButton
		for _, buttonText := range row {
			buttonRow = append(buttonRow, tgbotapi.NewKeyboardButton(buttonText))
		}
		buttons = append(buttons, buttonRow)
	}

	keyboardMarkup := tgbotapi.ReplyKeyboardMarkup{
		Keyboard:        buttons,
		ResizeKeyboard:  true,
		OneTimeKeyboard: false,
	}

	msg.ReplyMarkup = keyboardMarkup

	_, err := h.bot.Send(msg)
	if err != nil {
		h.logger.Error("ошибка отправки сообщения с клавиатурой",
			zap.Int64("chat_id", chatID),
			zap.Error(err))
		return err
	}

	return nil
}

// removeKeyboard убирает клавиатуру
func (h *Handler) removeKeyboard(chatID int64) error {
	msg := tgbotapi.NewMessage(chatID, "")
	msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)

	_, err := h.bot.Send(msg)
	if err != nil {
		h.logger.Error("ошибка удаления клавиатуры",
			zap.Int64("chat_id", chatID),
			zap.Error(err))
		return err
	}

	return nil
}

// sendErrorMessage отправляет сообщение об ошибке
func (h *Handler) sendErrorMessage(chatID int64, text string) error {
	return h.sendMessage(chatID, h.messages.Error(text))
}

// buildAIMessagesForAudio строит сообщения для AI из истории диалога для аудио сообщений
func (h *Handler) buildAIMessagesForAudio(messages []models.UserMessage, user *models.User) []ai.Message {
	var aiMessages []ai.Message

	// Добавляем специальный системный промпт для аудио
	systemPrompt := h.buildSystemPromptForAudio(user)
	aiMessages = append(aiMessages, ai.Message{
		Role:    "system",
		Content: systemPrompt,
	})

	// Добавляем историю диалога
	for _, msg := range messages {
		aiMessages = append(aiMessages, ai.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return aiMessages
}

// buildSystemPromptForAudio создает специальный системный промпт для аудио сообщений
func (h *Handler) buildSystemPromptForAudio(user *models.User) string {
	return h.prompts.GetAudioPrompt(user.Level)
}

// getLevelText возвращает текстовое представление уровня
func (h *Handler) getLevelText(level string) string {
	switch level {
	case models.LevelBeginner:
		return "Начинающий"
	case models.LevelIntermediate:
		return "Средний"
	case models.LevelAdvanced:
		return "Продвинутый"
	default:
		return "Начинающий"
	}
}

// getLevelEmoji возвращает эмодзи для уровня
func (h *Handler) getLevelEmoji(level string) string {
	switch level {
	case models.LevelBeginner:
		return "🔵"
	case models.LevelIntermediate:
		return "🟡"
	case models.LevelAdvanced:
		return "🟢"
	default:
		return "🔵"
	}
}

// extractTextFromHTML извлекает чистый текст из HTML, удаляя все теги

// extractEnglishFromResponse извлекает только английскую часть из ответа с переводом
func (h *Handler) extractEnglishFromResponse(responseWithTranslation string) string {
	// Ищем начало спойлера с переводом
	spoilerStart := strings.Index(responseWithTranslation, "<tg-spoiler>")
	if spoilerStart == -1 {
		// Если спойлера нет, возвращаем весь текст
		return strings.TrimSpace(responseWithTranslation)
	}

	// Извлекаем только английскую часть (до спойлера)
	englishPart := responseWithTranslation[:spoilerStart]
	return strings.TrimSpace(englishPart)
}

// cleanTextForTelegram очищает текст для корректного отображения в Telegram
func (h *Handler) cleanTextForTelegram(text string) string {
	// Очищаем текст от потенциально опасных HTML тегов
	// Оставляем только безопасные теги для HTML режима

	// Заменяем переносы строк
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = strings.ReplaceAll(text, "<br />", "\n")

	// Заменяем горизонтальные линии
	text = strings.ReplaceAll(text, "<hr>", "\n"+strings.Repeat("-", 20)+"\n")
	text = strings.ReplaceAll(text, "<hr/>", "\n"+strings.Repeat("-", 20)+"\n")
	text = strings.ReplaceAll(text, "<hr />", "\n"+strings.Repeat("-", 20)+"\n")

	// Удаляем div и p теги, оставляя содержимое
	text = strings.ReplaceAll(text, "<div>", "")
	text = strings.ReplaceAll(text, "</div>", "\n")
	text = strings.ReplaceAll(text, "<p>", "")
	text = strings.ReplaceAll(text, "</p>", "\n")

	// Удаляем span теги, оставляя содержимое
	text = strings.ReplaceAll(text, "<span>", "")
	text = strings.ReplaceAll(text, "</span>", "")

	// Удаляем лишние переносы строк
	text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	text = strings.ReplaceAll(text, "\n\n\n", "\n\n")

	return strings.TrimSpace(text)
}

// stripHTMLTags удаляет все HTML теги из текста и декодирует HTML-сущности
func (h *Handler) stripHTMLTags(text string) string {
	// Декодируем HTML-сущности
	text = html.UnescapeString(text)
	// Удаляем HTML теги
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(text, "")
}

// getOrCreateDialogContext получает или создает контекст диалога для пользователя
func (h *Handler) getOrCreateDialogContext(userID int64, level string) *DialogContext {
	if context, exists := h.dialogContexts[userID]; exists && !context.IsStale() {
		return context
	}

	// Создаем новый контекст с системным промптом
	var systemPrompt string
	if level == "beginner" {
		systemPrompt = h.prompts.GetEnglishMessagePrompt(level)
	} else if level == "intermediate" {
		systemPrompt = h.prompts.GetEnglishMessagePrompt(level)
	} else {
		systemPrompt = h.prompts.GetEnglishMessagePrompt(level)
	}

	context := NewDialogContext(userID, level, systemPrompt)
	h.dialogContexts[userID] = context
	return context
}

// cleanAIResponse очищает ответ AI от неподдерживаемых HTML-тегов
func (h *Handler) cleanAIResponse(text string) string {
	// Удаляем неподдерживаемые теги, оставляя содержимое
	text = strings.ReplaceAll(text, "<ul>", "")
	text = strings.ReplaceAll(text, "</ul>", "\n")
	text = strings.ReplaceAll(text, "<li>", "• ")
	text = strings.ReplaceAll(text, "</li>", "\n")
	text = strings.ReplaceAll(text, "<div>", "")
	text = strings.ReplaceAll(text, "</div>", "\n")
	text = strings.ReplaceAll(text, "<p>", "")
	text = strings.ReplaceAll(text, "</p>", "\n")
	text = strings.ReplaceAll(text, "<span>", "")
	text = strings.ReplaceAll(text, "</span>", "")

	// Заменяем Markdown разметку на HTML теги
	text = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(text, "<b>$1</b>")

	// Заменяем заголовки # на жирный текст
	text = regexp.MustCompile(`^#+\s+(.+)$`).ReplaceAllString(text, "<b>$1</b>")
	text = regexp.MustCompile(`\n#+\s+(.+)$`).ReplaceAllString(text, "\n<b>$1</b>")

	// Удаляем лишние символы разметки
	text = strings.ReplaceAll(text, "---", "")
	text = strings.ReplaceAll(text, "___", "")

	// Очищаем от лишних пробелов и переносов
	text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	text = strings.TrimSpace(text)

	return text
}

// handleAudioMessage обрабатывает голосовые и аудио сообщения
func (h *Handler) handleAudioMessage(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	// Проверяем лимит сообщений для бесплатных пользователей
	canSend, err := h.premiumService.CanSendMessage(ctx, user.ID)
	if err != nil {
		h.logger.Error("ошибка проверки лимита сообщений", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка проверки лимита сообщений")
	}

	if !canSend {
		return h.handleMessageLimit(ctx, message.Chat.ID, user)
	}

	// Обновляем study streak только раз в день
	h.updateStudyActivity(user)

	// Отправляем сообщение о начале обработки
	processingMsg := tgbotapi.NewMessage(message.Chat.ID, "🎤 Обрабатываю аудио сообщение...")
	processingMsg.ReplyToMessageID = message.MessageID
	_, err = h.bot.Send(processingMsg)
	if err != nil {
		h.logger.Error("ошибка отправки сообщения о обработке", zap.Error(err))
	}

	// Определяем тип аудио и получаем файл
	var fileID string
	var fileExt string

	if message.Voice != nil {
		fileID = message.Voice.FileID
		fileExt = ".ogg"
		// Проверяем размер голосового сообщения
		if message.Voice.FileSize > MaxFileSize {
			return h.sendErrorMessage(message.Chat.ID, "Файл слишком большой. Максимум 25MB.")
		}
	} else if message.Audio != nil {
		fileID = message.Audio.FileID
		fileExt = ".mp3"
		// Проверяем размер аудио файла
		if message.Audio.FileSize > MaxFileSize {
			return h.sendErrorMessage(message.Chat.ID, "Файл слишком большой. Максимум 25MB.")
		}
	} else {
		return h.sendErrorMessage(message.Chat.ID, "Неподдерживаемый тип аудио")
	}

	// Получаем файл от Telegram
	file, err := h.bot.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		h.logger.Error("ошибка получения файла от Telegram", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка получения аудио")
	}

	// Дополнительная проверка размера файла
	if !h.validateFileSize(file.FileSize) {
		return h.sendErrorMessage(message.Chat.ID, "Файл слишком большой или поврежден")
	}

	// Генерируем безопасное имя файла
	fileName, err := h.generateSecureFileName(fileExt)
	if err != nil {
		h.logger.Error("ошибка генерации имени файла", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка обработки аудио")
	}

	// Создаем безопасную папку для аудио файлов
	audioDir := filepath.Join(".", "temp", "audio")
	if err := os.MkdirAll(audioDir, 0750); err != nil {
		h.logger.Error("ошибка создания папки для аудио", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка обработки аудио")
	}

	// Создаем безопасный путь к файлу
	filePath := filepath.Join(audioDir, fileName)

	// Проверяем, что путь безопасен (защита от path traversal)
	if !strings.HasPrefix(filepath.Clean(filePath), filepath.Clean(audioDir)) {
		h.logger.Error("попытка path traversal атаки", zap.String("path", filePath))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка безопасности")
	}

	// Скачиваем файл с таймаутом
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", file.Link(h.bot.Token), nil)
	if err != nil {
		h.logger.Error("ошибка создания запроса", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка скачивания аудио")
	}

	resp, err := client.Do(req)
	if err != nil {
		h.logger.Error("ошибка скачивания файла", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка скачивания аудио")
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		h.logger.Error("неудачный статус скачивания", zap.Int("status", resp.StatusCode))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка скачивания аудио")
	}

	// Создаем файл с безопасными правами
	out, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0640)
	if err != nil {
		h.logger.Error("ошибка создания файла", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка сохранения аудио")
	}
	defer func() {
		out.Close()
		// Всегда удаляем временный файл
		if removeErr := os.Remove(filePath); removeErr != nil {
			h.logger.Warn("ошибка удаления временного файла", zap.Error(removeErr))
		}
	}()

	// Ограничиваем размер копируемых данных
	limitedReader := io.LimitReader(resp.Body, MaxFileSize)
	written, err := io.Copy(out, limitedReader)
	if err != nil {
		h.logger.Error("ошибка копирования файла", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка сохранения аудио")
	}

	// Проверяем, что файл не превышает лимит
	if written >= MaxFileSize {
		h.logger.Error("файл превысил максимальный размер", zap.Int64("size", written))
		return h.sendErrorMessage(message.Chat.ID, "Файл слишком большой")
	}

	// Закрываем файл перед транскрибацией
	if err := out.Close(); err != nil {
		h.logger.Error("ошибка закрытия файла", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка сохранения аудио")
	}

	// Транскрибируем аудио
	transcription, err := h.whisperClient.TranscribeFile(ctx, filePath)
	if err != nil {
		h.logger.Error("ошибка транскрибации", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка транскрибации")
	}

	// Проверяем, что транскрибация не пустая
	if transcription.Text == "" {
		return h.sendErrorMessage(message.Chat.ID, "Не удалось распознать речь")
	}

	// Отправляем результат транскрибации
	transcriptionMsg := fmt.Sprintf(
		"🎤 <b>Распознанная речь:</b>\n\n<blockquote>%s</blockquote>",
		transcription.Text,
	)
	msg := tgbotapi.NewMessage(message.Chat.ID, transcriptionMsg)
	msg.ParseMode = "HTML"
	msg.ReplyToMessageID = message.MessageID
	_, err = h.bot.Send(msg)
	if err != nil {
		h.logger.Error("ошибка отправки результата транскрибации", zap.Error(err))
		return err
	}

	// Сохраняем транскрибированный текст как сообщение пользователя
	_, err = h.messageService.SaveUserMessage(ctx, user.ID, transcription.Text)
	if err != nil {
		h.logger.Error("ошибка сохранения транскрибированного сообщения", zap.Error(err))
		// Не возвращаем ошибку, так как транскрибация уже отправлена
	}

	// Получаем историю диалога (оптимизировано для контекста)
	history, err := h.messageService.GetChatHistory(ctx, user.ID, ChatHistoryForAudio)
	if err != nil {
		h.logger.Error("ошибка получения истории диалога", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка получения истории диалога")
	}

	// Преобразуем сообщения в формат AI с специальным промптом для аудио
	aiMessages := h.buildAIMessagesForAudio(history.Messages, user)

	// Генерируем ответ с помощью AI (с автоматической санитизацией)
	options := ai.GenerationOptions{
		Temperature: 0.7,
		MaxTokens:   500,
	}
	response, err := h.aiClient.GenerateResponse(ctx, aiMessages, options)
	if err != nil {
		h.logger.Error("ошибка генерации ответа", zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка генерации ответа")
	}

	// Сохраняем ответ ассистента
	_, err = h.messageService.SaveAssistantMessage(ctx, user.ID, response.Content)
	if err != nil {
		h.logger.Error("ошибка сохранения ответа ассистента", zap.Error(err))
		// Не возвращаем ошибку, так как ответ уже отправлен
	}

	// Увеличиваем счетчик сообщений пользователя
	if err := h.premiumService.IncrementMessageCount(ctx, user.ID); err != nil {
		h.logger.Error("ошибка увеличения счетчика сообщений", zap.Error(err))
	}

	// Отправляем ответ
	return h.sendMessage(message.Chat.ID, response.Content)
}

// handleLevelTestCallback обрабатывает ответ на вопрос теста через callback
func (h *Handler) handleLevelTestCallback(ctx context.Context, callback *tgbotapi.CallbackQuery, user *models.User, answer int) error {
	levelTest, exists := h.activeLevelTests[user.ID]
	if !exists {
		return h.sendMessage(callback.Message.Chat.ID, "❌ Тест не найден. Начните новый тест.")
	}

	if levelTest.CurrentQuestion >= len(levelTest.Questions) {
		return h.completeLevelTest(ctx, callback.Message.Chat.ID, user)
	}

	currentQ := levelTest.Questions[levelTest.CurrentQuestion]
	isCorrect := answer == currentQ.CorrectAnswer
	points := 0
	if isCorrect {
		points = currentQ.Points
		levelTest.Score += points
	}

	// Сохраняем ответ
	levelTest.Answers = append(levelTest.Answers, models.LevelTestAnswer{
		QuestionID: currentQ.ID,
		Answer:     answer,
		IsCorrect:  isCorrect,
		Points:     points,
	})

	// Показываем результат ответа
	var feedback string
	if isCorrect {
		feedback = "✅ <b>Правильно!</b>"
	} else {
		correctOption := currentQ.Options[currentQ.CorrectAnswer]
		feedback = fmt.Sprintf("❌ <b>Неправильно.</b> Правильный ответ: <b>%d. %s</b>", currentQ.CorrectAnswer+1, correctOption)
	}

	// Редактируем сообщение с результатом
	editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID,
		fmt.Sprintf(`🎯 <b>Вопрос %d из %d</b>

%s

%s

⏳ <b>Переход к следующему вопросу...</b>`,
			levelTest.CurrentQuestion+1,
			len(levelTest.Questions),
			currentQ.Question,
			feedback))
	editMsg.ParseMode = "HTML"

	if _, err := h.bot.Send(editMsg); err != nil {
		h.logger.Error("ошибка редактирования сообщения теста", zap.Error(err))
	}

	// Переходим к следующему вопросу
	levelTest.CurrentQuestion++

	// Небольшая пауза перед следующим вопросом
	time.Sleep(2 * time.Second)

	return h.showCurrentQuestion(ctx, callback.Message.Chat.ID, user)
}

// handleTestCancelCallback обрабатывает отмену теста через callback
func (h *Handler) handleTestCancelCallback(ctx context.Context, callback *tgbotapi.CallbackQuery, user *models.User) error {
	// Удаляем активный тест
	delete(h.activeLevelTests, user.ID)

	// Сбрасываем состояние пользователя
	newState := models.StateIdle
	updateReq := &models.UpdateUserRequest{
		CurrentState: &newState,
	}
	_, err := h.userService.UpdateUser(ctx, user.ID, updateReq)
	if err != nil {
		h.logger.Error("ошибка обновления состояния пользователя", zap.Error(err))
	}

	// Обновляем локальные данные пользователя
	user.CurrentState = models.StateIdle

	// Записываем метрику отмены теста
	h.userMetrics.RecordXP(user.ID, 0, "level_test_cancelled")

	cancelMessage := `❌ <b>Тест отменен</b>

Тестирование завершено без результатов.

🎯 <b>Что дальше?</b>
• Попробуй пройти тест позже  
• Изучай английский в своём темпе  
• Используй команду "<b>🎯 Тест уровня</b>", когда будешь готов`

	// Редактируем сообщение
	editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, cancelMessage)
	editMsg.ParseMode = "HTML"
	editMsg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{},
	}

	if _, err := h.bot.Send(editMsg); err != nil {
		h.logger.Error("ошибка редактирования сообщения об отмене теста", zap.Error(err))
		// Если не удалось отредактировать, отправляем новое сообщение
		return h.sendMessageWithKeyboard(callback.Message.Chat.ID, cancelMessage, h.messages.GetMainKeyboard())
	}

	return nil
}

// handleLevelChangeCallback обрабатывает смену уровня пользователя
func (h *Handler) handleLevelChangeCallback(ctx context.Context, callback *tgbotapi.CallbackQuery, user *models.User, newLevel string) error {
	// Проверяем, что уровень валидный
	if !models.IsValidLevel(newLevel) {
		return h.sendMessage(callback.Message.Chat.ID, "❌ Некорректный уровень")
	}

	// Обновляем уровень пользователя
	updateReq := &models.UpdateUserRequest{
		Level: &newLevel,
	}
	_, err := h.userService.UpdateUser(ctx, user.ID, updateReq)
	if err != nil {
		h.logger.Error("ошибка обновления уровня пользователя", zap.Error(err))
		return h.sendMessage(callback.Message.Chat.ID, "❌ Ошибка обновления уровня")
	}

	// Обновляем локальные данные
	user.Level = newLevel

	successMessage := fmt.Sprintf(`✅ <b>Уровень изменен!</b>

📚 <b>Новый уровень:</b> %s

💡 <b>Теперь я буду:</b>
• Использовать подходящую сложность
• Давать соответствующие упражнения
• Подстраивать объяснения под твой уровень

🎯 <b>XP остается прежним:</b> %d XP

Продолжай изучать английский! 🚀`, h.getLevelText(newLevel), user.XP)

	// Редактируем сообщение
	editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, successMessage)
	editMsg.ParseMode = "HTML"
	editMsg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{},
	}

	if _, err := h.bot.Send(editMsg); err != nil {
		h.logger.Error("ошибка редактирования сообщения о смене уровня", zap.Error(err))
		// Если не удалось отредактировать, отправляем новое сообщение
		return h.sendMessageWithKeyboard(callback.Message.Chat.ID, successMessage, h.messages.GetMainKeyboard())
	}

	return nil
}

// handleKeepCurrentLevelCallback обрабатывает сохранение текущего уровня
func (h *Handler) handleKeepCurrentLevelCallback(ctx context.Context, callback *tgbotapi.CallbackQuery, user *models.User) error {
	keepMessage := fmt.Sprintf(`✅ <b>Уровень сохранен!</b>

📚 <b>Текущий уровень:</b> %s

💡 <b>Совет:</b> Ты можешь пройти тест еще раз завтра, если захочешь изменить уровень

🎯 Продолжай изучать английский! 🚀`, h.getLevelText(user.Level))

	// Редактируем сообщение
	editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, keepMessage)
	editMsg.ParseMode = "HTML"
	editMsg.ReplyMarkup = &tgbotapi.InlineKeyboardMarkup{
		InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{},
	}

	if _, err := h.bot.Send(editMsg); err != nil {
		h.logger.Error("ошибка редактирования сообщения о сохранении уровня", zap.Error(err))
		// Если не удалось отредактировать, отправляем новое сообщение
		return h.sendMessageWithKeyboard(callback.Message.Chat.ID, keepMessage, h.messages.GetMainKeyboard())
	}

	return nil
}

// generateLevelTest создает новый тест уровня для пользователя
func (h *Handler) generateLevelTest(userID int64) *models.LevelTest {
	// Выбираем 10 случайных вопросов из разных уровней
	questions := h.selectRandomQuestions(10)

	maxScore := 0
	for _, q := range questions {
		maxScore += q.Points
	}

	return &models.LevelTest{
		UserID:          userID,
		CurrentQuestion: 0,
		Questions:       questions,
		Answers:         make([]models.LevelTestAnswer, 0),
		Score:           0,
		MaxScore:        maxScore,
		StartedAt:       time.Now(),
	}
}

// calculateLevel определяет уровень пользователя на основе результатов теста
func (h *Handler) calculateLevel(score, maxScore int) (string, string) {
	percentage := float64(score) / float64(maxScore) * 100

	if percentage >= 80 {
		return models.LevelAdvanced, "Отличный результат! Ты владеешь английским на продвинутом уровне. Можешь изучать сложные темы и общаться на любые темы."
	} else if percentage >= 60 {
		return models.LevelIntermediate, "Хороший результат! Ты владеешь английским на среднем уровне. Можешь изучать более сложные темы и улучшать разговорные навыки."
	} else {
		return models.LevelBeginner, "Хорошее начало! Ты владеешь английским на начальном уровне. Стоит изучать основы грамматики и базовую лексику."
	}
}

// selectRandomQuestions выбирает случайные вопросы из разных уровней
func (h *Handler) selectRandomQuestions(count int) []models.LevelTestQuestion {
	// Здесь будут вопросы для теста
	questions := []models.LevelTestQuestion{
		// Beginner Level Questions
		{
			ID:            1,
			Question:      "What is the correct form of 'to be' in this sentence?\n'I ___ a student.'",
			Options:       []string{"am", "is", "are", "be"},
			CorrectAnswer: 0,
			Level:         models.LevelBeginner,
			Points:        1,
		},
		{
			ID:            2,
			Question:      "Choose the correct article:\n'I have ___ apple.'",
			Options:       []string{"a", "an", "the", "no article"},
			CorrectAnswer: 1,
			Level:         models.LevelBeginner,
			Points:        1,
		},
		{
			ID:            3,
			Question:      "What is the plural form of 'child'?",
			Options:       []string{"childs", "children", "childrens", "child"},
			CorrectAnswer: 1,
			Level:         models.LevelBeginner,
			Points:        1,
		},
		{
			ID:            4,
			Question:      "Complete the sentence:\n'She ___ to school every day.'",
			Options:       []string{"go", "goes", "going", "went"},
			CorrectAnswer: 1,
			Level:         models.LevelBeginner,
			Points:        1,
		},
		// Intermediate Level Questions
		{
			ID:            5,
			Question:      "Choose the correct tense:\n'I ___ English for three years.'",
			Options:       []string{"learn", "am learning", "have been learning", "learned"},
			CorrectAnswer: 2,
			Level:         models.LevelIntermediate,
			Points:        2,
		},
		{
			ID:            6,
			Question:      "Which sentence is correct?",
			Options:       []string{"If I would have money, I would buy a car.", "If I had money, I would buy a car.", "If I have money, I would buy a car.", "If I will have money, I would buy a car."},
			CorrectAnswer: 1,
			Level:         models.LevelIntermediate,
			Points:        2,
		},
		{
			ID:            7,
			Question:      "Choose the correct preposition:\n'She is interested ___ music.'",
			Options:       []string{"in", "on", "at", "for"},
			CorrectAnswer: 0,
			Level:         models.LevelIntermediate,
			Points:        2,
		},
		// Advanced Level Questions
		{
			ID:            8,
			Question:      "Choose the correct form:\n'I wish I ___ more time to finish the project.'",
			Options:       []string{"have", "had", "would have", "will have"},
			CorrectAnswer: 1,
			Level:         models.LevelAdvanced,
			Points:        3,
		},
		{
			ID:            9,
			Question:      "Which sentence uses the subjunctive mood correctly?",
			Options:       []string{"I suggest that he comes early.", "I suggest that he come early.", "I suggest that he will come early.", "I suggest that he is coming early."},
			CorrectAnswer: 1,
			Level:         models.LevelAdvanced,
			Points:        3,
		},
		{
			ID:            10,
			Question:      "Choose the sentence with correct inversion:\n'Never before ___ such a beautiful sunset.'",
			Options:       []string{"I have seen", "have I seen", "I had seen", "had I seen"},
			CorrectAnswer: 1,
			Level:         models.LevelAdvanced,
			Points:        3,
		},
	}

	// Возвращаем все вопросы (можно добавить логику перемешивания)
	return questions
}

// sendLevelUpNotification отправляет уведомление о повышении уровня
func (h *Handler) sendLevelUpNotification(userID int64, oldLevel, newLevel string, totalXP int) {
	// Получаем информацию о следующем уровне
	xpForNext, _ := models.GetXPForNextLevel(totalXP)

	var levelEmoji string
	var levelDescription string

	switch newLevel {
	case models.LevelIntermediate:
		levelEmoji = "🟡"
		levelDescription = "Средний уровень! Теперь ты можешь изучать более сложные темы и улучшать разговорные навыки."
	case models.LevelAdvanced:
		levelEmoji = "🟢"
		levelDescription = "Продвинутый уровень! Ты отлично владеешь английским и можешь изучать сложные темы."
	default:
		levelEmoji = "🔵"
		levelDescription = "Начальный уровень. Продолжай изучать основы!"
	}

	var message string
	if newLevel == models.LevelAdvanced {
		message = fmt.Sprintf(`🎉 <b>ПОЗДРАВЛЯЕМ!</b> %s

🆙 <b>Уровень повышен!</b>
%s → <b>%s %s</b>

⭐ Общий опыт: <b>%d XP</b>

🎯 %s

🏆 <b>Ты достиг максимального уровня!</b> Продолжай общаться и совершенствуй свой английский!`,
			levelEmoji,
			h.getLevelText(oldLevel),
			levelEmoji,
			h.getLevelText(newLevel),
			totalXP,
			levelDescription)
	} else {
		message = fmt.Sprintf(`🎉 <b>ПОЗДРАВЛЯЕМ!</b> %s

🆙 <b>Уровень повышен!</b>
%s → <b>%s %s</b>

⭐ Общий опыт: <b>%d XP</b>
🎯 До следующего уровня: <b>%d XP</b>

💡 %s`,
			levelEmoji,
			h.getLevelText(oldLevel),
			levelEmoji,
			h.getLevelText(newLevel),
			totalXP,
			xpForNext,
			levelDescription)
	}

	// Отправляем уведомление (используем контекст с таймаутом)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Получаем пользователя для отправки сообщения
	user, err := h.userService.GetUserByTelegramID(ctx, userID)
	if err != nil {
		h.logger.Error("ошибка получения пользователя для уведомления",
			zap.Error(err),
			zap.Int64("user_id", userID))
		return
	}

	err = h.sendMessage(user.TelegramID, message)
	if err != nil {
		h.logger.Error("ошибка отправки уведомления о повышении уровня",
			zap.Error(err),
			zap.Int64("user_id", userID))
	}
}

// / handleLeaderboardButton показывает рейтинг пользователей прямо в Telegram
func (h *Handler) handleLeaderboardButton(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	// Получаем топ пользователей (с большим лимитом для статистики)
	users, err := h.userService.GetTopUsersByStreak(ctx, 100)
	if err != nil {
		h.logger.Error("ошибка получения пользователей для рейтинга",
			zap.Error(err))
		return h.sendErrorMessage(message.Chat.ID, "Ошибка загрузки рейтинга")
	}

	var leaderboardText strings.Builder

	// Заголовок
	leaderboardText.WriteString("🏆 <b>Рейтинг пользователей Lingua AI</b>\n\n")

	// Общая статистика
	leaderboardText.WriteString("📊 <b>Общая статистика</b>\n")
	leaderboardText.WriteString(fmt.Sprintf("👥 Всего пользователей: <b>%d</b>\n", len(users)))

	// Активные за сегодня
	today := time.Now().Truncate(24 * time.Hour)
	activeToday := 0
	for _, u := range users {
		if u.LastSeen.After(today) {
			activeToday++
		}
	}
	leaderboardText.WriteString(fmt.Sprintf("🔥 Активны сегодня: <b>%d</b>\n\n", activeToday))

	// Топ-10 пользователей
	topN := 10
	if len(users) < topN {
		topN = len(users)
	}
	leaderboardText.WriteString("🥇 <b>Топ-10 пользователей</b>\n\n")

	for i, u := range users[:topN] {
		rank := i + 1
		rankIcon := ""
		switch rank {
		case 1:
			rankIcon = "🥇"
		case 2:
			rankIcon = "🥈"
		case 3:
			rankIcon = "🥉"
		default:
			rankIcon = fmt.Sprintf("№%d", rank)
		}

		// Имя + username (скрываем часть username)
		username := u.FirstName
		if u.Username != "" {
			hiddenUsername := h.hideUsername(u.Username)
			username += fmt.Sprintf(" (@%s)", hiddenUsername)
		}

		// Формат строки
		leaderboardText.WriteString(fmt.Sprintf(
			"%s <b>%s</b>\n   %s %s • 🔥 %d дн. • ⭐ <b>%d XP</b>\n\n",
			rankIcon, username,
			h.getLevelEmoji(u.Level),
			h.getLevelText(u.Level),
			u.StudyStreak,
			u.XP,
		))
	}

	// Позиция текущего пользователя
	for i, u := range users {
		if u.ID == user.ID {
			leaderboardText.WriteString("📍 <b>Твоя позиция</b>\n")
			leaderboardText.WriteString(fmt.Sprintf(
				"   №%d • %s %s • ⭐ <b>%d XP</b>\n",
				i+1,
				h.getLevelEmoji(user.Level),
				h.getLevelText(user.Level),
				user.XP,
			))
			break
		}
	}

	// Отправляем сообщение
	msg := tgbotapi.NewMessage(message.Chat.ID, leaderboardText.String())
	msg.ParseMode = "HTML"

	if _, err := h.bot.Send(msg); err != nil {
		h.logger.Error("ошибка отправки рейтинга",
			zap.Error(err),
			zap.Int64("chat_id", message.Chat.ID))
		return err
	}

	return nil
}

// handleLearningCommand обрабатывает команду /learning
func (h *Handler) handleLearningCommand(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	return h.handleLearningButton(ctx, message, user)
}

// handleLearningButton обрабатывает кнопку "Обучение"
func (h *Handler) handleLearningButton(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	messageText := `📚 <b>Обучение</b>

Выберите способ изучения английского языка:

🎯 <b>Доступные методы:</b>
📝 Словарные карточки — изучение новых слов с интервальным повторением
🎓 Тест уровня — определите свой текущий уровень английского

Что хотите попробовать?`

	return h.sendMessageWithKeyboard(message.Chat.ID, messageText, h.messages.GetLearningKeyboard())
}

// handleMainHelpCallback обрабатывает callback для помощи
func (h *Handler) handleMainHelpCallback(ctx context.Context, callback *tgbotapi.CallbackQuery, user *models.User) error {
	helpText := `❓ <b>Помощь по использованию бота</b>

🤖 <b>Основные команды:</b>
/start — Начать работу с ботом
/help — Показать это сообщение
/learning — Открыть меню обучения
/stats — Показать вашу статистику
/premium — Информация о премиум-подписке

📚 <b>Функции обучения:</b>
• Отправляйте текстовые сообщения для перевода
• Отправляйте голосовые сообщения для транскрибации
• Изучайте слова с помощью карточек
• Проходите тесты для определения уровня

💡 <b>Советы:</b>
• Регулярно занимайтесь для лучшего результата
• Используйте карточки для запоминания слов
• Отправляйте голосовые сообщения для практики произношения

Если у вас есть вопросы, обратитесь к администратору.`

	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, helpText)
	msg.ParseMode = "HTML"

	_, err := h.bot.Send(msg)
	return err
}

// handleMainPremiumCallback обрабатывает callback для премиума
func (h *Handler) handleMainPremiumCallback(ctx context.Context, callback *tgbotapi.CallbackQuery, user *models.User) error {
	return h.handlePremiumCommand(ctx, callback.Message, user)
}

// handleMainRatingCallback обрабатывает callback для рейтинга
func (h *Handler) handleMainRatingCallback(ctx context.Context, callback *tgbotapi.CallbackQuery, user *models.User) error {
	// Используем существующую функцию для показа рейтинга
	return h.handleLeaderboardButton(ctx, callback.Message, user)
}

// handleLearningMenuCallback обрабатывает callback для меню обучения
func (h *Handler) handleLearningMenuCallback(ctx context.Context, callback *tgbotapi.CallbackQuery, user *models.User) error {
	return h.handleLearningButton(ctx, callback.Message, user)
}

// handleMainStatsCallback обрабатывает callback для статистики
func (h *Handler) handleMainStatsCallback(ctx context.Context, callback *tgbotapi.CallbackQuery, user *models.User) error {
	// Используем существующую функцию Stats из Messages
	levelText := h.getLevelText(user.Level)
	lastStudyDate := user.LastStudyDate.Format("02.01.2006")

	messageText := h.messages.Stats(user.FirstName, levelText, user.XP, user.StudyStreak, lastStudyDate)

	msg := tgbotapi.NewMessage(callback.Message.Chat.ID, messageText)
	msg.ParseMode = "HTML"

	_, err := h.bot.Send(msg)
	return err
}

// handleReferralButton обрабатывает нажатие кнопки "Реферальная ссылка"
func (h *Handler) handleReferralButton(ctx context.Context, message *tgbotapi.Message, user *models.User) error {
	// Получаем или генерируем реферальный код
	referralCode, err := h.referralService.GetOrGenerateReferralCode(ctx, user.ID)
	if err != nil {
		h.logger.Error("ошибка получения реферального кода", zap.Error(err))
		return h.sendMessage(message.Chat.ID, "Ошибка получения реферальной ссылки. Попробуйте позже.")
	}

	// Получаем статистику рефералов
	stats, err := h.referralService.GetReferralStats(ctx, user.ID)
	if err != nil {
		h.logger.Error("ошибка получения статистики рефералов", zap.Error(err))
		// Не возвращаем ошибку, показываем ссылку без статистики
	}

	// Формируем дополнительную информацию о статусе премиума
	var premiumStatus string
	if user.IsPremium {
		if user.PremiumExpiresAt != nil {
			premiumStatus = fmt.Sprintf("🌟 <b>У вас уже есть премиум до %s</b>\n\n💡 <em>Продолжайте приглашать друзей! Премиум будет продлен на месяц при достижении 10 рефералов.</em>",
				user.PremiumExpiresAt.Format("02.01.2006"))
		} else {
			premiumStatus = "🌟 <b>У вас уже есть активная премиум-подписка</b>\n\n💡 <em>Продолжайте приглашать друзей! Премиум будет продлен на месяц при достижении 10 рефералов.</em>"
		}
	} else {
		if stats != nil && stats.TotalReferrals >= 10 {
			premiumStatus = "🎉 <b>Поздравляем! У вас 10+ рефералов!</b>\n\n✅ <em>Премиум уже предоставлен на месяц.</em>"
		} else if stats != nil {
			remaining := 10 - stats.TotalReferrals
			premiumStatus = fmt.Sprintf("📈 <b>До премиума осталось: %d рефералов</b>\n\n💪 <em>Продолжайте приглашать друзей!</em>", remaining)
		} else {
			premiumStatus = "📈 <b>До премиума нужно: 10 рефералов</b>\n\n💪 <em>Начните приглашать друзей прямо сейчас!</em>"
		}
	}

	// Формируем сообщение
	var messageText string
	if stats != nil {
		messageText = fmt.Sprintf(`🔗 <b>Ваша реферальная ссылка</b>

📱 <b>Поделитесь этой ссылкой с друзьями:</b>
<code>https://t.me/%s?start=ref_%s</code>

📊 <b>Ваша статистика:</b>
• Приглашено друзей: <b>%d</b>
• Завершено регистраций: <b>%d</b>
• Ожидают регистрации: <b>%d</b>

🎁 <b>Награда:</b>
За <b>10 приглашенных друзей</b> вы получите <b>премиум на месяц</b>!
<em>(только если у вас еще нет активной премиум-подписки)</em>

%s

💡 <b>Как это работает:</b>
1. Отправьте ссылку другу
2. Друг переходит по ссылке и регистрируется
3. Вы получаете +1 к счетчику приглашений
4. При 10 приглашениях — премиум на месяц!`,
			h.bot.Self.UserName, referralCode, stats.TotalReferrals, stats.CompletedReferrals, stats.PendingReferrals, premiumStatus)
	} else {
		messageText = fmt.Sprintf(`🔗 <b>Ваша реферальная ссылка</b>

📱 <b>Поделитесь этой ссылкой с друзьями:</b>
<code>https://t.me/%s?start=ref_%s</code>

🎁 <b>Награда:</b>
За <b>10 приглашенных друзей</b> вы получите <b>премиум на месяц</b>!
<em>(только если у вас еще нет активной премиум-подписки)</em>

%s

💡 <b>Как это работает:</b>
1. Отправьте ссылку другу
2. Друг переходит по ссылку и регистрируется
3. Вы получаете +1 к счетчику приглашений
4. При 10 приглашениях — премиум на месяц!`,
			h.bot.Self.UserName, referralCode, premiumStatus)
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, messageText)
	msg.ParseMode = "HTML"

	_, err = h.bot.Send(msg)
	return err
}

// hideUsername скрывает часть username для приватности
func (h *Handler) hideUsername(username string) string {
	if len(username) <= 3 {
		// Если username очень короткий, скрываем все кроме первого символа
		return string(username[0]) + strings.Repeat("*", len(username)-1)
	}

	// Показываем первые 2-3 символа и последние 2-3 символа, остальное звездочки
	showStart := 2
	showEnd := 2

	// Если username короткий, показываем меньше
	if len(username) <= 6 {
		showStart = 1
		showEnd = 1
	}

	// Если username очень длинный, показываем больше
	if len(username) > 10 {
		showStart = 3
		showEnd = 3
	}

	// Создаем строку со звездочками в середине
	hidden := username[:showStart] + strings.Repeat("*", len(username)-showStart-showEnd) + username[len(username)-showEnd:]
	return hidden
}

// handleTTSCallback обрабатывает запрос на озвучку текста
func (h *Handler) handleTTSCallback(ctx context.Context, callback *tgbotapi.CallbackQuery, user *models.User, text string) error {
	h.logger.Info("обработка TTS callback", zap.String("text", text))

	// Проверяем, что TTS сервис доступен
	if h.ttsService == nil {
		msg := tgbotapi.NewCallback(callback.ID, "❌ Озвучка временно недоступна")
		h.bot.Request(msg)
		return nil
	}

	// Отправляем уведомление о начале генерации
	msg := tgbotapi.NewCallback(callback.ID, "🎵 Генерирую аудио...")
	h.bot.Request(msg)

	// Генерируем аудио
	audioData, err := h.ttsService.SynthesizeText(ctx, text)
	if err != nil {
		h.logger.Error("ошибка генерации TTS", zap.Error(err))
		msg := tgbotapi.NewCallback(callback.ID, "❌ Ошибка генерации аудио")
		h.bot.Request(msg)
		return err
	}

	// Отправляем аудио
	audio := tgbotapi.NewAudio(callback.Message.Chat.ID, tgbotapi.FileBytes{
		Name:  "tts_audio.wav",
		Bytes: audioData,
	})
	audio.Caption = "🔊 Озвучка: " + text

	if _, err := h.bot.Send(audio); err != nil {
		h.logger.Error("ошибка отправки аудио", zap.Error(err))
		return err
	}

	h.logger.Info("TTS аудио отправлено", zap.String("text", text))
	return nil
}

// createTTSButton создает кнопку для озвучки текста
func (h *Handler) createTTSButton(text string) tgbotapi.InlineKeyboardButton {
	// Кодируем текст в base64 для передачи в callback
	encodedText := base64.StdEncoding.EncodeToString([]byte(text))
	return tgbotapi.NewInlineKeyboardButtonData("🔊 Озвучить", "tts_"+encodedText)
}

// sendMessageWithTTS отправляет сообщение с кнопкой озвучки (если TTS включен)
func (h *Handler) sendMessageWithTTS(chatID int64, text string) error {
	h.logger.Info("🔍 sendMessageWithTTS вызван", zap.String("text", text), zap.Bool("tts_enabled", h.ttsService != nil))

	// Если TTS отключен, отправляем обычное сообщение
	if h.ttsService == nil {
		h.logger.Info("🔍 TTS отключен, отправляем обычное сообщение")
		return h.sendMessage(chatID, text)
	}

	// Извлекаем английский текст из ответа AI
	englishText := h.extractEnglishText(text)
	h.logger.Info("🔍 extractEnglishText результат", zap.String("original", text), zap.String("extracted", englishText))
	if englishText == "" {
		// Если английского текста нет, отправляем обычное сообщение
		h.logger.Info("🔍 Английский текст не найден, отправляем обычное сообщение")
		return h.sendMessage(chatID, text)
	}

	// Создаем кнопку озвучки
	ttsButton := h.createTTSButton(englishText)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(ttsButton),
	)

	// Отправляем сообщение с кнопкой
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = keyboard
	msg.ParseMode = "HTML"

	if _, err := h.bot.Send(msg); err != nil {
		h.logger.Error("ошибка отправки сообщения с TTS", zap.Error(err))
		return err
	}

	return nil
}

// extractEnglishText извлекает английский текст из ответа AI
func (h *Handler) extractEnglishText(text string) string {
	h.logger.Info("🔍 extractEnglishText вызван", zap.String("text", text))

	// 1. Ищем первую строку с английским текстом (до эмодзи флага)
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Пропускаем пустые строки и строки с эмодзи флагами
		if line == "" || strings.Contains(line, "🇷🇺") || strings.Contains(line, "🇺🇸") {
			continue
		}
		// Если строка содержит английские буквы, возвращаем её
		if h.containsEnglish(line) {
			h.logger.Info("🔍 Найден английский текст в строке", zap.String("line", line))
			return line
		}
	}

	// 2. Ищем текст в кавычках
	if strings.Contains(text, "\"") {
		start := strings.Index(text, "\"")
		end := strings.LastIndex(text, "\"")
		if start != -1 && end != -1 && end > start {
			quoted := text[start+1 : end]
			// Проверяем, что это английский текст (содержит латинские буквы)
			if h.containsEnglish(quoted) {
				h.logger.Info("🔍 Найден английский текст в кавычках", zap.String("quoted", quoted))
				return quoted
			}
		}
	}

	// 3. Ищем текст после двоеточия
	if strings.Contains(text, ":") {
		parts := strings.Split(text, ":")
		if len(parts) > 1 {
			afterColon := strings.TrimSpace(parts[1])
			// Берем первую строку после двоеточия
			lines := strings.Split(afterColon, "\n")
			if len(lines) > 0 && h.containsEnglish(lines[0]) {
				h.logger.Info("🔍 Найден английский текст после двоеточия", zap.String("after_colon", lines[0]))
				return strings.TrimSpace(lines[0])
			}
		}
	}

	// 4. Если ничего не найдено, возвращаем весь текст если он содержит английские буквы
	if h.containsEnglish(text) {
		h.logger.Info("🔍 Возвращаем весь текст как английский", zap.String("text", text))
		return text
	}

	h.logger.Info("🔍 Английский текст не найден")
	return ""
}

// containsEnglish проверяет, содержит ли текст английские буквы
func (h *Handler) containsEnglish(text string) bool {
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}
