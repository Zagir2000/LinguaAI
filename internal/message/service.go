package message

import (
	"context"
	"fmt"

	"lingua-ai/internal/store"
	"lingua-ai/pkg/models"

	"go.uber.org/zap"
)

// Service представляет сервис для работы с сообщениями
type Service struct {
	store  store.Store
	logger *zap.Logger
}

// NewService создает новый сервис сообщений
func NewService(store store.Store, logger *zap.Logger) *Service {
	return &Service{
		store:  store,
		logger: logger,
	}
}

// CreateMessage создает новое сообщение с автоматической очисткой старых
func (s *Service) CreateMessage(ctx context.Context, req *models.CreateMessageRequest) (*models.UserMessage, error) {
	// Валидация роли
	if !models.IsValidRole(req.Role) {
		return nil, fmt.Errorf("некорректная роль сообщения: %s", req.Role)
	}

	// Проверяем, существует ли пользователь
	_, err := s.store.User().GetByID(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("пользователь не найден: %w", err)
	}

	// Создаем сообщение
	message := &models.UserMessage{
		UserID:  req.UserID,
		Role:    req.Role,
		Content: req.Content,
	}

	// Используем новый метод с автоочисткой
	if err := s.store.Message().CreateWithCleanup(ctx, message); err != nil {
		return nil, fmt.Errorf("ошибка создания сообщения: %w", err)
	}

	s.logger.Debug("создано новое сообщение с автоочисткой",
		zap.Int64("message_id", message.ID),
		zap.Int64("user_id", req.UserID),
		zap.String("role", req.Role))

	return message, nil
}

// GetChatHistory получает историю диалога пользователя
func (s *Service) GetChatHistory(ctx context.Context, userID int64, limit int) (*models.ChatHistory, error) {
	if limit <= 0 {
		limit = 20 // Значение по умолчанию
	}
	if limit > 100 {
		limit = 100 // Максимальный лимит
	}

	history, err := s.store.Message().GetChatHistory(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения истории диалога: %w", err)
	}

	return history, nil
}

// ClearChatHistory очищает историю диалога пользователя
func (s *Service) ClearChatHistory(ctx context.Context, userID int64) error {
	if err := s.store.Message().DeleteByUserID(ctx, userID); err != nil {
		return fmt.Errorf("ошибка очистки истории диалога: %w", err)
	}

	s.logger.Info("очищена история диалога пользователя",
		zap.Int64("user_id", userID))

	return nil
}

// SaveUserMessage сохраняет сообщение пользователя
func (s *Service) SaveUserMessage(ctx context.Context, userID int64, content string) (*models.UserMessage, error) {
	req := &models.CreateMessageRequest{
		UserID:  userID,
		Role:    models.RoleUser,
		Content: content,
	}

	return s.CreateMessage(ctx, req)
}

// SaveAssistantMessage сохраняет сообщение ассистента
func (s *Service) SaveAssistantMessage(ctx context.Context, userID int64, content string) (*models.UserMessage, error) {
	req := &models.CreateMessageRequest{
		UserID:  userID,
		Role:    models.RoleAssistant,
		Content: content,
	}

	return s.CreateMessage(ctx, req)
}

// GetMessageCount получает количество сообщений пользователя
func (s *Service) GetMessageCount(ctx context.Context, userID int64) (int, error) {
	count, err := s.store.Message().GetMessageCount(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("ошибка получения количества сообщений: %w", err)
	}

	return count, nil
}

// CleanupOldMessages удаляет старые сообщения пользователя, оставляя только последние N
func (s *Service) CleanupOldMessages(ctx context.Context, userID int64, keepCount int) error {
	if err := s.store.Message().CleanupOldMessages(ctx, userID, keepCount); err != nil {
		return fmt.Errorf("ошибка очистки старых сообщений: %w", err)
	}

	s.logger.Info("очищены старые сообщения пользователя",
		zap.Int64("user_id", userID),
		zap.Int("keep_count", keepCount))

	return nil
}

// GetUserMessageStats получает статистику сообщений пользователя
func (s *Service) GetUserMessageStats(ctx context.Context, userID int64) (*MessageStats, error) {
	messages, err := s.store.Message().GetByUserID(ctx, userID, 1000)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения сообщений: %w", err)
	}

	stats := &MessageStats{
		TotalMessages:     len(messages),
		UserMessages:      0,
		AssistantMessages: 0,
	}

	for _, msg := range messages {
		switch msg.Role {
		case models.RoleUser:
			stats.UserMessages++
		case models.RoleAssistant:
			stats.AssistantMessages++
		}
	}

	return stats, nil
}

// MessageStats представляет статистику сообщений
type MessageStats struct {
	TotalMessages     int `json:"total_messages"`
	UserMessages      int `json:"user_messages"`
	AssistantMessages int `json:"assistant_messages"`
}
