package premium

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"lingua-ai/pkg/models"
)

// Service представляет сервис для работы с премиум-подпиской
type Service struct {
	userRepo    UserRepository
	paymentRepo PaymentRepository
	logger      *zap.Logger
	yukassa     YukassaClient
}

// UserRepository интерфейс для работы с пользователями
type UserRepository interface {
	GetByID(ctx context.Context, id int64) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	IncrementMessagesCount(ctx context.Context, userID int64) error
}

// PaymentRepository интерфейс для работы с платежами
type PaymentRepository interface {
	Create(ctx context.Context, payment *models.Payment) error
	GetByPaymentID(ctx context.Context, paymentID string) (*models.Payment, error)
	Update(ctx context.Context, payment *models.Payment) error
}

// YukassaClient интерфейс для работы с YooKassa API
type YukassaClient interface {
	CreatePayment(ctx context.Context, amount float64, currency string, description string) (string, string, error)
	CheckPaymentStatus(ctx context.Context, paymentID string) (string, error)
}

// NewService создает новый сервис премиум-подписки
func NewService(userRepo UserRepository, paymentRepo PaymentRepository, yukassa YukassaClient, logger *zap.Logger) *Service {
	return &Service{
		userRepo:    userRepo,
		paymentRepo: paymentRepo,
		yukassa:     yukassa,
		logger:      logger,
	}
}

// GetPremiumPlans возвращает доступные планы премиум-подписки
func (s *Service) GetPremiumPlans() []models.PremiumPlan {
	return []models.PremiumPlan{
		{
			ID:           1,
			Name:         "Месяц",
			DurationDays: 30,
			Price:        1.0,
			Currency:     "RUB",
			Description:  "Премиум-подписка на 1 месяц",
			Features: []string{
				"Безлимитные сообщения",
				"Приоритетная поддержка",
				"Расширенные упражнения",
				"Персональные рекомендации",
			},
		},
		{
			ID:           2,
			Name:         "3 месяца",
			DurationDays: 90,
			Price:        2.0,
			Currency:     "RUB",
			Description:  "Премиум-подписка на 3 месяца (экономия 20%)",
			Features: []string{
				"Безлимитные сообщения",
				"Приоритетная поддержка",
				"Расширенные упражнения",
				"Персональные рекомендации",
				"Скидка 20%",
			},
		},
		{
			ID:           3,
			Name:         "Год",
			DurationDays: 365,
			Price:        3.0,
			Currency:     "RUB",
			Description:  "Премиум-подписка на 1 год (экономия 30%)",
			Features: []string{
				"Безлимитные сообщения",
				"Приоритетная поддержка",
				"Расширенные упражнения",
				"Персональные рекомендации",
				"Скидка 30%",
				"Эксклюзивные материалы",
			},
		},
	}
}

// CreatePayment создает новый платеж через YooKassa API
func (s *Service) CreatePayment(ctx context.Context, userID int64, planID int) (*models.Payment, string, string, error) {
	// Получаем план премиум-подписки
	plans := s.GetPremiumPlans()
	var selectedPlan *models.PremiumPlan
	for _, plan := range plans {
		if plan.ID == planID {
			selectedPlan = &plan
			break
		}
	}

	if selectedPlan == nil {
		return nil, "", "", fmt.Errorf("план с ID %d не найден", planID)
	}

	// Создаем платеж через YooKassa
	paymentID, confirmationURL, err := s.yukassa.CreatePayment(ctx, selectedPlan.Price, selectedPlan.Currency, selectedPlan.Description)
	if err != nil {
		return nil, "", "", fmt.Errorf("ошибка создания платежа в YooKassa: %w", err)
	}

	// Создаем запись о платеже в базе данных
	payment := &models.Payment{
		PaymentID:           paymentID,
		UserID:              userID,
		Amount:              selectedPlan.Price,
		Currency:            selectedPlan.Currency,
		Status:              "pending",
		PremiumDurationDays: selectedPlan.DurationDays,
		CreatedAt:           time.Now(),
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, "", "", fmt.Errorf("ошибка сохранения платежа в базе данных: %w", err)
	}

	s.logger.Info("платеж создан",
		zap.String("payment_id", paymentID),
		zap.Int64("user_id", userID),
		zap.Int("plan_id", planID),
		zap.Float64("amount", selectedPlan.Price))

	return payment, paymentID, confirmationURL, nil
}

// ProcessPaymentCallback обрабатывает callback от YooKassa
func (s *Service) ProcessPaymentCallback(ctx context.Context, paymentID string, status string) error {
	// Получаем платеж из базы данных
	payment, err := s.paymentRepo.GetByPaymentID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("ошибка получения платежа: %w", err)
	}

	// Обновляем статус платежа
	payment.Status = status
	if err := s.paymentRepo.Update(ctx, payment); err != nil {
		return fmt.Errorf("ошибка обновления статуса платежа: %w", err)
	}

	// Если платеж успешен, активируем премиум
	if status == "succeeded" {
		// Используем длительность из платежа
		if err := s.activatePremium(ctx, payment.UserID, payment.PremiumDurationDays); err != nil {
			s.logger.Error("ошибка активации премиума после успешного платежа",
				zap.String("payment_id", paymentID),
				zap.Int64("user_id", payment.UserID),
				zap.Error(err))
			return fmt.Errorf("ошибка активации премиума: %w", err)
		}
	}

	s.logger.Info("платеж обработан",
		zap.String("payment_id", paymentID),
		zap.String("status", status),
		zap.Int64("user_id", payment.UserID))

	return nil
}

// ActivatePremium активирует премиум-подписку для пользователя (публичный метод)
func (s *Service) ActivatePremium(ctx context.Context, userID int64, durationDays int) error {
	return s.activatePremium(ctx, userID, durationDays)
}

// GetPaymentByID получает платеж по ID
func (s *Service) GetPaymentByID(ctx context.Context, paymentID string) (*models.Payment, error) {
	return s.paymentRepo.GetByPaymentID(ctx, paymentID)
}

// UpdatePayment обновляет платеж
func (s *Service) UpdatePayment(ctx context.Context, payment *models.Payment) error {
	return s.paymentRepo.Update(ctx, payment)
}

// activatePremium активирует премиум-подписку для пользователя
func (s *Service) activatePremium(ctx context.Context, userID int64, durationDays int) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("ошибка получения пользователя: %w", err)
	}

	// Устанавливаем премиум-статус
	user.IsPremium = true

	// Вычисляем дату истечения
	expiresAt := time.Now().AddDate(0, 0, durationDays)
	user.PremiumExpiresAt = &expiresAt

	// Убираем лимит на сообщения
	user.MaxMessages = 0

	// Обновляем пользователя
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("ошибка обновления пользователя: %w", err)
	}

	s.logger.Info("премиум-подписка активирована",
		zap.Int64("user_id", userID),
		zap.Int("duration_days", durationDays),
		zap.Time("expires_at", expiresAt))

	return nil
}

// CheckPremiumStatus проверяет статус премиум-подписки пользователя
func (s *Service) CheckPremiumStatus(ctx context.Context, userID int64) (*models.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения пользователя: %w", err)
	}

	// Проверяем, не истекла ли премиум-подписка
	if user.IsPremium && user.PremiumExpiresAt != nil {
		if time.Now().After(*user.PremiumExpiresAt) {
			// Премиум истек, деактивируем
			user.IsPremium = false
			user.PremiumExpiresAt = nil
			user.MaxMessages = 50 // Возвращаем лимит

			if err := s.userRepo.Update(ctx, user); err != nil {
				s.logger.Error("ошибка деактивации премиума", zap.Error(err), zap.Int64("user_id", userID))
			} else {
				s.logger.Info("премиум-подписка деактивирована", zap.Int64("user_id", userID))
			}
		}
	}

	return user, nil
}

// CanSendMessage проверяет, может ли пользователь отправить сообщение
func (s *Service) CanSendMessage(ctx context.Context, userID int64) (bool, error) {
	// Сначала проверяем и сбрасываем счетчик, если прошел день
	if err := s.resetDailyCounterIfNeeded(ctx, userID); err != nil {
		s.logger.Error("ошибка сброса дневного счетчика при проверке лимита", zap.Error(err), zap.Int64("user_id", userID))
		return false, err
	}

	user, err := s.CheckPremiumStatus(ctx, userID)
	if err != nil {
		return false, err
	}

	// Премиум пользователи могут отправлять неограниченное количество сообщений
	if user.IsPremium {
		return true, nil
	}

	// Бесплатные пользователи ограничены лимитом
	return user.MessagesCount < user.MaxMessages, nil
}

// IncrementMessageCount увеличивает счетчик сообщений пользователя
func (s *Service) IncrementMessageCount(ctx context.Context, userID int64) error {
	s.logger.Info("увеличиваем счетчик сообщений в PremiumService", zap.Int64("user_id", userID))

	err := s.userRepo.IncrementMessagesCount(ctx, userID)
	if err != nil {
		s.logger.Error("ошибка увеличения счетчика сообщений в PremiumService",
			zap.Error(err), zap.Int64("user_id", userID))
	} else {
		s.logger.Info("счетчик сообщений успешно увеличен в PremiumService", zap.Int64("user_id", userID))
	}

	return err
}

// resetDailyCounterIfNeeded сбрасывает счетчик сообщений, если прошел день
func (s *Service) resetDailyCounterIfNeeded(ctx context.Context, userID int64) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("ошибка получения пользователя: %w", err)
	}

	today := time.Now().Truncate(24 * time.Hour)
	resetDate := user.MessagesResetDate.Truncate(24 * time.Hour)

	// Если дата сброса не сегодня, сбрасываем счетчик
	if !resetDate.Equal(today) {
		s.logger.Info("сбрасываем дневной счетчик сообщений",
			zap.Int64("user_id", userID),
			zap.Time("last_reset", resetDate),
			zap.Time("today", today))

		user.MessagesCount = 0
		user.MessagesResetDate = today

		if err := s.userRepo.Update(ctx, user); err != nil {
			return fmt.Errorf("ошибка обновления пользователя при сбросе счетчика: %w", err)
		}
	}

	return nil
}

// GetUserStats возвращает статистику пользователя по сообщениям
func (s *Service) GetUserStats(ctx context.Context, userID int64) (map[string]any, error) {
	// Сначала проверяем и сбрасываем счетчик, если прошел день
	if err := s.resetDailyCounterIfNeeded(ctx, userID); err != nil {
		s.logger.Error("ошибка сброса дневного счетчика при получении статистики", zap.Error(err), zap.Int64("user_id", userID))
		return nil, err
	}

	// Получаем пользователя после возможного сброса
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения пользователя: %w", err)
	}

	// Проверяем статус премиума без изменения данных
	isPremium := user.IsPremium
	maxMessages := user.MaxMessages

	if user.IsPremium && user.PremiumExpiresAt != nil {
		if time.Now().After(*user.PremiumExpiresAt) {
			// Премиум истек, но не изменяем данные здесь
			isPremium = false
			maxMessages = 50
		}
	}

	stats := map[string]any{
		"is_premium":         isPremium,
		"messages_count":     user.MessagesCount,
		"max_messages":       maxMessages,
		"remaining_messages": 0,
	}

	if isPremium {
		stats["remaining_messages"] = "∞"
		if user.PremiumExpiresAt != nil {
			stats["remaining_messages"] = "∞"
			stats["premium_expires_at"] = user.PremiumExpiresAt.Format("02.01.2006")
		}
	} else {
		stats["remaining_messages"] = maxMessages - user.MessagesCount
	}

	return stats, nil
}
