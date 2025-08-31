package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"lingua-ai/internal/premium"
	"lingua-ai/internal/store"
	"lingua-ai/internal/user"
	"lingua-ai/pkg/models"
)

// WebhookHandler обрабатывает webhook'и от Telegram
type WebhookHandler struct {
	telegramService *TelegramPaymentService
	userService     UserService
	paymentService  PaymentService
	store           store.Store // Добавляем store для транзакций
}

// NewWebhookHandler создает новый обработчик webhook'ов
func NewWebhookHandler(telegramService *TelegramPaymentService, userService UserService, paymentService PaymentService, store store.Store) *WebhookHandler {
	return &WebhookHandler{
		telegramService: telegramService,
		userService:     userService,
		paymentService:  paymentService,
		store:           store,
	}
}

// HandleWebhook обрабатывает входящий webhook
func (h *WebhookHandler) HandleWebhook(update []byte) error {
	var webhookUpdate struct {
		UpdateID         int64             `json:"update_id"`
		ShippingQuery    *ShippingQuery    `json:"shipping_query,omitempty"`
		PreCheckoutQuery *PreCheckoutQuery `json:"pre_checkout_query,omitempty"`
		Message          *struct {
			SuccessfulPayment *SuccessfulPayment `json:"successful_payment,omitempty"`
		} `json:"message,omitempty"`
	}

	if err := json.Unmarshal(update, &webhookUpdate); err != nil {
		return fmt.Errorf("ошибка парсинга webhook: %w", err)
	}

	// Обрабатываем различные типы обновлений
	if webhookUpdate.ShippingQuery != nil {
		return h.handleShippingQuery(webhookUpdate.ShippingQuery)
	}

	if webhookUpdate.PreCheckoutQuery != nil {
		return h.handlePreCheckoutQuery(webhookUpdate.PreCheckoutQuery)
	}

	if webhookUpdate.Message != nil && webhookUpdate.Message.SuccessfulPayment != nil {
		return h.handleSuccessfulPayment(webhookUpdate.Message.SuccessfulPayment)
	}

	return nil
}

// handleShippingQuery обрабатывает запрос о доставке
func (h *WebhookHandler) handleShippingQuery(query *ShippingQuery) error {
	log.Printf("Получен запрос о доставке от пользователя %d", query.From.ID)

	// Создаем варианты доставки
	shippingOptions := h.telegramService.CreateShippingOptions()

	// Отвечаем на запрос
	return h.telegramService.AnswerShippingQuery(
		query.ID,
		true,
		shippingOptions,
		"",
	)
}

// handlePreCheckoutQuery обрабатывает предварительную проверку платежа
func (h *WebhookHandler) handlePreCheckoutQuery(query *PreCheckoutQuery) error {
	log.Printf("Получена предварительная проверка платежа от пользователя %d", query.From.ID)

	// Проверяем payload
	if !strings.HasPrefix(query.InvoicePayload, "premium_") {
		return h.telegramService.AnswerPreCheckoutQuery(
			query.ID,
			false,
			"Неверный тип платежа",
		)
	}

	// Парсим payload: premium_USERID_PLAN_DURATION
	parts := strings.Split(query.InvoicePayload, "_")
	if len(parts) != 4 {
		return h.telegramService.AnswerPreCheckoutQuery(
			query.ID,
			false,
			"Неверный формат payload",
		)
	}

	userID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return h.telegramService.AnswerPreCheckoutQuery(
			query.ID,
			false,
			"Неверный ID пользователя",
		)
	}

	plan := parts[2]
	durationDays, err := strconv.Atoi(parts[3])
	if err != nil {
		return h.telegramService.AnswerPreCheckoutQuery(
			query.ID,
			false,
			"Неверная длительность подписки",
		)
	}

	// Проверяем, что пользователь существует
	_, err = h.userService.GetUserByTelegramID(userID)
	if err != nil {
		return h.telegramService.AnswerPreCheckoutQuery(
			query.ID,
			false,
			"Пользователь не найден",
		)
	}

	// Проверяем, что план валиден
	if !h.isValidPlan(plan, durationDays) {
		return h.telegramService.AnswerPreCheckoutQuery(
			query.ID,
			false,
			"Неверный план подписки",
		)
	}

	// Подтверждаем платеж
	return h.telegramService.AnswerPreCheckoutQuery(query.ID, true, "")
}

// handleSuccessfulPayment обрабатывает успешный платеж
func (h *WebhookHandler) handleSuccessfulPayment(payment *SuccessfulPayment) error {
	log.Printf("Получен успешный платеж: %s", payment.ProviderPaymentChargeID)

	// Парсим payload (формат: "premium_USERID_PLAN_DURATION")
	parts := strings.Split(payment.InvoicePayload, "_")
	if len(parts) != 4 {
		return fmt.Errorf("неверный формат payload: %s, ожидается 4 части", payment.InvoicePayload)
	}

	// Проверяем префикс
	if parts[0] != "premium" {
		return fmt.Errorf("неверный префикс payload: %s, ожидается 'premium'", parts[0])
	}

	userID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return fmt.Errorf("неверный ID пользователя: %s", parts[1])
	}

	plan := parts[2]
	durationDays, err := strconv.Atoi(parts[3])
	if err != nil {
		return fmt.Errorf("неверная длительность: %s", parts[3])
	}

	// Получаем пользователя
	user, err := h.userService.GetUserByTelegramID(userID)
	if err != nil {
		return fmt.Errorf("пользователь не найден: %w", err)
	}

	// Создаем запись о платеже
	completedAt := time.Now()
	paymentRecord := &PaymentRecord{
		UserID:              user.ID,
		Amount:              payment.TotalAmount,
		Currency:            payment.Currency,
		PaymentID:           payment.ProviderPaymentChargeID,
		Status:              "completed",
		PremiumDurationDays: durationDays,
		CreatedAt:           time.Now(),
		CompletedAt:         &completedAt,
		Metadata: map[string]interface{}{
			"plan":                plan,
			"telegram_payment_id": payment.TelegramPaymentChargeID,
			"provider_payment_id": payment.ProviderPaymentChargeID,
		},
	}

	// Выполняем все операции в транзакции
	return h.executeInTransaction(context.Background(), func(txStore store.Store) error {
		// Сохраняем платеж
		if err := h.paymentService.CreatePayment(paymentRecord); err != nil {
			return fmt.Errorf("ошибка сохранения платежа: %w", err)
		}

		// Активируем премиум подписку
		if err := h.userService.ActivatePremium(user.ID, durationDays); err != nil {
			return fmt.Errorf("ошибка активации премиума: %w", err)
		}

		log.Printf("Премиум подписка активирована для пользователя %d на %d дней", user.ID, durationDays)
		return nil
	})
}

// isValidPlan проверяет валидность плана подписки
func (h *WebhookHandler) isValidPlan(plan string, durationDays int) bool {
	validPlans := map[string][]int{
		"month":   {30, 31},
		"quarter": {90, 91, 92},
		"year":    {365, 366},
		"custom":  {1, 7, 14, 30, 60, 90, 180, 365},
	}

	if validDurations, exists := validPlans[plan]; exists {
		for _, validDuration := range validDurations {
			if validDuration == durationDays {
				return true
			}
		}
	}

	return false
}

// Интерфейсы для сервисов
type UserService interface {
	GetUserByTelegramID(telegramID int64) (*User, error)
	ActivatePremium(userID int64, durationDays int) error
}

type PaymentService interface {
	CreatePayment(payment *PaymentRecord) error
}

// Адаптеры для существующих сервисов
type UserServiceAdapter struct {
	userService *user.Service
}

func NewUserServiceAdapter(userService *user.Service) UserService {
	return &UserServiceAdapter{userService: userService}
}

func (a *UserServiceAdapter) GetUserByTelegramID(telegramID int64) (*User, error) {
	// Получаем пользователя по Telegram ID через userService
	user, err := a.userService.GetUserByTelegramID(context.Background(), telegramID)
	if err != nil {
		return nil, fmt.Errorf("пользователь не найден: %w", err)
	}

	return &User{
		ID:        user.ID,
		IsBot:     false,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Username:  user.Username,
	}, nil
}

func (a *UserServiceAdapter) ActivatePremium(userID int64, durationDays int) error {
	// Активируем премиум через userService
	user, err := a.userService.GetByID(context.Background(), userID)
	if err != nil {
		return fmt.Errorf("ошибка получения пользователя: %w", err)
	}

	// Устанавливаем премиум статус
	user.IsPremium = true

	// Вычисляем дату истечения
	expiresAt := time.Now().AddDate(0, 0, durationDays)
	user.PremiumExpiresAt = &expiresAt

	// Убираем лимит на сообщения
	user.MaxMessages = 0

	// Обновляем пользователя
	if err := a.userService.Update(context.Background(), user); err != nil {
		return fmt.Errorf("ошибка обновления пользователя: %w", err)
	}

	log.Printf("Премиум активирован для пользователя %d на %d дней, истекает %s",
		userID, durationDays, expiresAt.Format("2006-01-02"))

	return nil
}

type PaymentServiceAdapter struct {
	premiumService *premium.Service
	paymentRepo    store.PaymentRepository
}

func NewPaymentServiceAdapter(premiumService *premium.Service, paymentRepo store.PaymentRepository) PaymentService {
	return &PaymentServiceAdapter{
		premiumService: premiumService,
		paymentRepo:    paymentRepo,
	}
}

func (a *PaymentServiceAdapter) CreatePayment(payment *PaymentRecord) error {
	// Конвертируем сумму в зависимости от валюты
	var amount float64
	switch payment.Currency {
	case "RUB":
		amount = float64(payment.Amount) / 100.0 // Копейки в рубли
	case "USD":
		amount = float64(payment.Amount) / 100.0 // Центы в доллары
	case "EUR":
		amount = float64(payment.Amount) / 100.0 // Евроценты в евро
	default:
		amount = float64(payment.Amount) // Для других валют оставляем как есть
	}

	// Создаем models.Payment из PaymentRecord
	modelsPayment := &models.Payment{
		UserID:              payment.UserID,
		Amount:              amount,
		Currency:            payment.Currency,
		PaymentID:           payment.PaymentID,
		Status:              payment.Status,
		PremiumDurationDays: payment.PremiumDurationDays,
		CreatedAt:           payment.CreatedAt,
		CompletedAt:         payment.CompletedAt,
		Metadata:            payment.Metadata,
	}

	// Создаем платеж через store
	if err := a.paymentRepo.Create(context.Background(), modelsPayment); err != nil {
		return fmt.Errorf("ошибка создания платежа в БД: %w", err)
	}

	log.Printf("Платеж создан: ID=%d, UserID=%d, Amount=%.2f %s, Status=%s",
		modelsPayment.ID, modelsPayment.UserID, modelsPayment.Amount, modelsPayment.Currency, modelsPayment.Status)

	return nil
}

// PaymentRecord представляет запись о платеже
type PaymentRecord struct {
	ID                  int64
	UserID              int64
	Amount              int
	Currency            string
	PaymentID           string
	Status              string
	PremiumDurationDays int
	CreatedAt           time.Time
	CompletedAt         *time.Time
	Metadata            map[string]interface{}
}

// executeInTransaction выполняет операции в транзакции
func (h *WebhookHandler) executeInTransaction(ctx context.Context, fn func(store.Store) error) error {
	// Начинаем транзакцию
	tx, err := h.store.DB().Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}

	// Выполняем функцию с основным store
	// TODO: В будущем нужно создать правильный транзакционный store
	if err := fn(h.store); err != nil {
		// Если произошла ошибка, откатываем транзакцию
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			log.Printf("ошибка отката транзакции: %v", rollbackErr)
		}
		return fmt.Errorf("ошибка в транзакции: %w", err)
	}

	// Если все успешно, фиксируем транзакцию
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("ошибка фиксации транзакции: %w", err)
	}

	return nil
}
