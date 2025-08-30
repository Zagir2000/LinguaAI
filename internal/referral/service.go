package referral

import (
	"context"
	"fmt"
	"time"

	"lingua-ai/internal/store"
	"lingua-ai/pkg/models"

	"go.uber.org/zap"
)

// Service представляет сервис для управления реферальной системой
type Service struct {
	referralRepo store.ReferralRepository
	userRepo     store.UserRepository
	logger       *zap.Logger
}

// NewService создает новый сервис рефералов
func NewService(referralRepo store.ReferralRepository, userRepo store.UserRepository, logger *zap.Logger) *Service {
	return &Service{
		referralRepo: referralRepo,
		userRepo:     userRepo,
		logger:       logger,
	}
}

// GetOrGenerateReferralCode получает существующий или генерирует новый реферальный код
func (s *Service) GetOrGenerateReferralCode(ctx context.Context, userID int64) (string, error) {
	// Получаем пользователя
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("ошибка получения пользователя: %w", err)
	}

	// Если код уже есть, возвращаем его
	if user.ReferralCode != nil {
		return *user.ReferralCode, nil
	}

	// Генерируем уникальный код с проверкой
	maxAttempts := 10
	var code string

	for attempt := 0; attempt < maxAttempts; attempt++ {
		generatedCode, err := s.referralRepo.GenerateReferralCode(ctx)
		if err != nil {
			return "", fmt.Errorf("ошибка генерации реферального кода: %w", err)
		}

		// Проверяем, что код уникален
		existingUser, err := s.referralRepo.GetUserByReferralCode(ctx, generatedCode)
		if err != nil {
			// Код уникален, можно использовать
			code = generatedCode
			break
		}

		if existingUser == nil {
			// Код уникален, можно использовать
			code = generatedCode
			break
		}

		// Код уже существует, пробуем снова
		s.logger.Warn("сгенерированный код уже существует, пробуем снова",
			zap.String("code", generatedCode),
			zap.Int("attempt", attempt+1))
	}

	if code == "" {
		return "", fmt.Errorf("не удалось сгенерировать уникальный реферальный код после %d попыток", maxAttempts)
	}

	// Обновляем пользователя с новым кодом
	user.ReferralCode = &code
	if err := s.userRepo.Update(ctx, user); err != nil {
		return "", fmt.Errorf("ошибка обновления пользователя: %w", err)
	}

	return code, nil
}

// CreateReferral создает новую реферальную связь
func (s *Service) CreateReferral(ctx context.Context, referrerID, referredID int64) error {
	// Проверяем, что пользователи разные
	if referrerID == referredID {
		return fmt.Errorf("пользователь не может пригласить сам себя")
	}

	// Проверяем, что приглашенный пользователь еще не был приглашен
	existingReferral, err := s.referralRepo.GetReferralByReferredID(ctx, referredID)
	if err == nil && existingReferral != nil {
		return fmt.Errorf("пользователь уже был приглашен")
	}

	// Создаем реферальную связь
	referral := &models.Referral{
		ReferrerID: referrerID,
		ReferredID: referredID,
		Status:     string(models.ReferralStatusPending),
		CreatedAt:  time.Now(),
	}

	err = s.referralRepo.CreateReferral(ctx, referral)
	if err != nil {
		return fmt.Errorf("ошибка создания реферала: %w", err)
	}

	// Обновляем поле referred_by у приглашенного пользователя
	referredUser, err := s.userRepo.GetByID(ctx, referredID)
	if err != nil {
		s.logger.Error("ошибка получения приглашенного пользователя", zap.Error(err))
		// Не возвращаем ошибку, так как реферал уже создан
	} else {
		referredUser.ReferredBy = &referrerID
		if err := s.userRepo.Update(ctx, referredUser); err != nil {
			s.logger.Error("ошибка обновления referred_by", zap.Error(err))
			// Не возвращаем ошибку, так как реферал уже создан
		}
	}

	// Обновляем счетчик referral_count у реферера
	referrerUser, err := s.userRepo.GetByID(ctx, referrerID)
	if err != nil {
		s.logger.Error("ошибка получения реферера", zap.Error(err))
		// Не возвращаем ошибку, так как реферал уже создан
	} else {
		referrerUser.ReferralCount++

		// Проверяем, достиг ли пользователь 10 приглашений для получения премиума
		if referrerUser.ReferralCount >= 10 && !referrerUser.IsPremium {
			// Предоставляем премиум на месяц
			referrerUser.IsPremium = true
			premiumExpiry := time.Now().AddDate(0, 1, 0) // +1 месяц
			referrerUser.PremiumExpiresAt = &premiumExpiry

			s.logger.Info("пользователь получил премиум за рефералы",
				zap.Int64("user_id", referrerID),
				zap.Int("referral_count", referrerUser.ReferralCount))
		}

		if err := s.userRepo.Update(ctx, referrerUser); err != nil {
			s.logger.Error("ошибка обновления referral_count", zap.Error(err))
			// Не возвращаем ошибку, так как реферал уже создан
		}
	}

	s.logger.Info("создан новый реферал",
		zap.Int64("referrer_id", referrerID),
		zap.Int64("referred_id", referredID))

	return nil
}

// ActivateReferral активирует реферал (когда приглашенный пользователь становится активным)
func (s *Service) ActivateReferral(ctx context.Context, referredID int64) error {
	// Получаем реферал
	referral, err := s.referralRepo.GetReferralByReferredID(ctx, referredID)
	if err != nil {
		return fmt.Errorf("реферал не найден: %w", err)
	}

	// Проверяем, что статус еще не завершен
	if referral.Status == string(models.ReferralStatusCompleted) {
		return fmt.Errorf("реферал уже активирован")
	}

	// Обновляем статус на completed
	now := time.Now()
	err = s.referralRepo.UpdateReferralStatus(ctx, referral.ID, string(models.ReferralStatusCompleted), &now)
	if err != nil {
		return fmt.Errorf("ошибка активации реферала: %w", err)
	}

	s.logger.Info("реферал активирован",
		zap.Int64("referral_id", referral.ID),
		zap.Int64("referred_id", referredID))

	return nil
}

// GetReferralStats получает статистику рефералов пользователя
func (s *Service) GetReferralStats(ctx context.Context, userID int64) (*models.ReferralStats, error) {
	stats, err := s.referralRepo.GetReferralStats(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения статистики рефералов: %w", err)
	}

	return stats, nil
}

// GetReferralLink формирует полную реферальную ссылку
func (s *Service) GetReferralLink(ctx context.Context, userID int64, botUsername string) (string, error) {
	code, err := s.GetOrGenerateReferralCode(ctx, userID)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("https://t.me/%s?start=ref_%s", botUsername, code), nil
}

// ValidateReferralCode проверяет валидность реферального кода
func (s *Service) ValidateReferralCode(ctx context.Context, referralCode string) (*models.User, error) {
	// Убираем префикс "ref_" если он есть
	if len(referralCode) > 3 && referralCode[:4] == "ref_" {
		referralCode = referralCode[4:]
	}

	// Получаем пользователя по коду
	user, err := s.referralRepo.GetUserByReferralCode(ctx, referralCode)
	if err != nil {
		return nil, fmt.Errorf("неверный реферальный код")
	}

	return user, nil
}

// CancelReferral отменяет реферал
func (s *Service) CancelReferral(ctx context.Context, referralID int64) error {
	err := s.referralRepo.UpdateReferralStatus(ctx, referralID, string(models.ReferralStatusCancelled), nil)
	if err != nil {
		return fmt.Errorf("ошибка отмены реферала: %w", err)
	}

	s.logger.Info("реферал отменен", zap.Int64("referral_id", referralID))
	return nil
}
