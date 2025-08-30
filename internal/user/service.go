package user

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lingua-ai/internal/store"
	"lingua-ai/pkg/models"

	"go.uber.org/zap"
)

// Service представляет сервис для работы с пользователями
type Service struct {
	store  store.Store
	logger *zap.Logger
}

// NewService создает новый сервис пользователей
func NewService(store store.Store, logger *zap.Logger) *Service {
	return &Service{
		store:  store,
		logger: logger,
	}
}

// CreateUser создает нового пользователя
func (s *Service) CreateUser(ctx context.Context, req *models.CreateUserRequest) (*models.User, error) {
	// Проверяем, существует ли пользователь
	existingUser, err := s.store.User().GetByTelegramID(ctx, req.TelegramID)
	if err == nil && existingUser != nil {
		return existingUser, nil // Пользователь уже существует
	}

	// Создаем нового пользователя
	user := &models.User{
		TelegramID: req.TelegramID,
		Username:   req.Username,
		FirstName:  req.FirstName,
		LastName:   req.LastName,
		Level:      models.LevelBeginner,
		XP:         0,
	}

	if err := s.store.User().Create(ctx, user); err != nil {
		return nil, fmt.Errorf("ошибка создания пользователя: %w", err)
	}

	s.logger.Info("создан новый пользователь",
		zap.Int64("telegram_id", req.TelegramID),
		zap.String("username", req.Username))

	return user, nil
}

// GetUserByTelegramID получает пользователя по Telegram ID
func (s *Service) GetUserByTelegramID(ctx context.Context, telegramID int64) (*models.User, error) {
	user, err := s.store.User().GetByTelegramID(ctx, telegramID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения пользователя: %w", err)
	}

	// Обновляем время последнего посещения
	if err := s.store.User().UpdateLastSeen(ctx, user.ID); err != nil {
		s.logger.Warn("не удалось обновить время последнего посещения",
			zap.Int64("user_id", user.ID),
			zap.Error(err))
	}

	return user, nil
}

// UpdateUser обновляет пользователя
func (s *Service) UpdateUser(ctx context.Context, userID int64, req *models.UpdateUserRequest) (*models.User, error) {
	// Получаем текущего пользователя
	user, err := s.store.User().GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения пользователя: %w", err)
	}

	// Обновляем поля, если они переданы
	if req.Level != nil {
		if !models.IsValidLevel(*req.Level) {
			return nil, fmt.Errorf("некорректный уровень: %s", *req.Level)
		}
		user.Level = *req.Level
	}

	if req.XP != nil {
		user.XP = *req.XP
	}

	if req.CurrentState != nil {
		if !models.IsValidState(*req.CurrentState) {
			return nil, fmt.Errorf("некорректное состояние: %s", *req.CurrentState)
		}
		user.CurrentState = *req.CurrentState
	}

	if req.LastSeen != nil {
		user.LastSeen = *req.LastSeen
	}

	// Обновляем premium поля, если они переданы
	if req.IsPremium != nil {
		user.IsPremium = *req.IsPremium
	}
	if req.PremiumExpiresAt != nil {
		user.PremiumExpiresAt = req.PremiumExpiresAt
	}
	if req.MessagesCount != nil {
		user.MessagesCount = *req.MessagesCount
	}
	if req.MaxMessages != nil {
		user.MaxMessages = *req.MaxMessages
	}
	if req.MessagesResetDate != nil {
		user.MessagesResetDate = *req.MessagesResetDate
	}
	if req.LastTestDate != nil {
		user.LastTestDate = req.LastTestDate
	}

	// Сохраняем изменения
	err = s.store.User().Update(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("ошибка обновления пользователя: %w", err)
	}

	s.logger.Info("пользователь обновлен",
		zap.Int64("user_id", userID),
		zap.String("level", user.Level),
		zap.Int("xp", user.XP),
		zap.Bool("is_premium", user.IsPremium),
		zap.Int("messages_count", user.MessagesCount),
		zap.Int("max_messages", user.MaxMessages))

	return user, nil
}

// AddXP добавляет опыт пользователю
func (s *Service) AddXP(ctx context.Context, userID int64, xp int) error {
	if xp <= 0 {
		return fmt.Errorf("XP должен быть положительным")
	}

	user, err := s.store.User().GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("ошибка получения пользователя: %w", err)
	}

	user.XP += xp

	// Проверяем, нужно ли повысить уровень
	oldLevel := user.Level
	user.Level = s.calculateLevel(user.XP)

	if err := s.store.User().Update(ctx, user); err != nil {
		return fmt.Errorf("ошибка обновления пользователя: %w", err)
	}

	s.logger.Info("добавлен XP пользователю",
		zap.Int64("user_id", userID),
		zap.Int("added_xp", xp),
		zap.Int("total_xp", user.XP),
		zap.String("old_level", oldLevel),
		zap.String("new_level", user.Level))

	return nil
}

// GetUserStats получает статистику пользователя
func (s *Service) GetUserStats(ctx context.Context, userID int64) (*models.UserStats, error) {
	stats, err := s.store.User().GetStats(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения статистики: %w", err)
	}

	return stats, nil
}

// calculateLevel рассчитывает уровень пользователя на основе XP
func (s *Service) calculateLevel(xp int) string {
	switch {
	case xp < 100:
		return models.LevelBeginner
	case xp < 500:
		return models.LevelIntermediate
	default:
		return models.LevelAdvanced
	}
}

// GetOrCreateUser получает пользователя или создает нового
func (s *Service) GetOrCreateUser(ctx context.Context, telegramID int64, username, firstName, lastName string) (*models.User, error) {
	// Пытаемся получить существующего пользователя
	user, err := s.store.User().GetByTelegramID(ctx, telegramID)
	if err == nil && user != nil {
		// Обновляем время последнего посещения
		if err := s.store.User().UpdateLastSeen(ctx, user.ID); err != nil {
			s.logger.Warn("не удалось обновить время последнего посещения",
				zap.Int64("user_id", user.ID),
				zap.Error(err))
		}
		return user, nil
	}

	// Если пользователь не найден, создаем нового
	if err != nil {
		// Проверяем, что это действительно ошибка "не найден"
		if !strings.Contains(err.Error(), "не найден") && !strings.Contains(err.Error(), "no rows") {
			// Логируем ошибку, но продолжаем создание
			s.logger.Warn("ошибка получения пользователя, создаем нового",
				zap.Int64("telegram_id", telegramID),
				zap.Error(err))
		}
	}

	// Создаем нового пользователя
	req := &models.CreateUserRequest{
		TelegramID: telegramID,
		Username:   username,
		FirstName:  firstName,
		LastName:   lastName,
	}

	return s.CreateUser(ctx, req)
}

// GetUserByID получает пользователя по ID
func (s *Service) GetUserByID(ctx context.Context, userID int64) (*models.User, error) {
	user, err := s.store.User().GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения пользователя: %w", err)
	}
	return user, nil
}

// UpdateStudyActivity обновляет активность обучения пользователя
func (s *Service) UpdateStudyActivity(ctx context.Context, userID int64) error {
	err := s.store.User().UpdateStudyActivity(ctx, userID)
	if err != nil {
		return fmt.Errorf("ошибка обновления активности обучения: %w", err)
	}
	return nil
}

// GetTopUsersByStreak получает топ пользователей по study streak
func (s *Service) GetTopUsersByStreak(ctx context.Context, limit int) ([]*models.User, error) {
	users, err := s.store.User().GetTopUsersByStreak(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения топ пользователей: %w", err)
	}
	return users, nil
}

// GetAllUsers получает всех пользователей для рейтинга
func (s *Service) GetAllUsers(ctx context.Context) ([]*models.User, error) {
	users, err := s.store.User().GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения всех пользователей: %w", err)
	}
	return users, nil
}

// GetInactiveUsers получает пользователей, неактивных более указанного времени
func (s *Service) GetInactiveUsers(ctx context.Context, inactiveDuration time.Duration) ([]*models.User, error) {
	users, err := s.store.User().GetInactiveUsers(ctx, inactiveDuration)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения неактивных пользователей: %w", err)
	}
	return users, nil
}

// GetByID получает пользователя по ID (для интерфейса premium.UserRepository)
func (s *Service) GetByID(ctx context.Context, id int64) (*models.User, error) {
	return s.store.User().GetByID(ctx, id)
}

// Update обновляет пользователя (для интерфейса premium.UserRepository)
func (s *Service) Update(ctx context.Context, user *models.User) error {
	// Создаем UpdateUserRequest из user
	updateReq := &models.UpdateUserRequest{
		Level:         &user.Level,
		XP:            &user.XP,
		LastSeen:      &user.LastSeen,
		MessagesCount: &user.MessagesCount,
		MaxMessages:   &user.MaxMessages,
	}

	// Всегда устанавливаем CurrentState, по умолчанию "idle"
	currentState := user.CurrentState
	if currentState == "" {
		currentState = "idle"
	}
	updateReq.CurrentState = &currentState

	// Всегда добавляем premium поля
	updateReq.IsPremium = &user.IsPremium
	updateReq.PremiumExpiresAt = user.PremiumExpiresAt
	updateReq.MessagesResetDate = &user.MessagesResetDate
	updateReq.LastTestDate = user.LastTestDate

	_, err := s.UpdateUser(ctx, user.ID, updateReq)
	return err
}

// IncrementMessagesCount увеличивает счетчик сообщений пользователя (для интерфейса premium.UserRepository)
func (s *Service) IncrementMessagesCount(ctx context.Context, userID int64) error {
	// Получаем пользователя
	user, err := s.store.User().GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Увеличиваем счетчик сообщений
	user.MessagesCount++

	// Обновляем пользователя
	return s.Update(ctx, user)
}
