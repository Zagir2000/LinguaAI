package store

import (
	"context"
	"fmt"
	"time"

	"lingua-ai/pkg/models"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Константы для управления историей сообщений
const (
	MaxMessagesPerUser = 10 // Максимальное количество сообщений на пользователя
)

// messageRepository реализует MessageRepository
type messageRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewMessageRepository создает новый репозиторий сообщений
func NewMessageRepository(db *pgxpool.Pool, logger *zap.Logger) MessageRepository {
	return &messageRepository{
		db:     db,
		logger: logger,
	}
}

// Create создает новое сообщение
func (r *messageRepository) Create(ctx context.Context, msg *models.UserMessage) error {
	query := `
		INSERT INTO user_messages (user_id, role, content, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id`

	msg.CreatedAt = time.Now()

	err := r.db.QueryRow(ctx, query,
		msg.UserID, msg.Role, msg.Content, msg.CreatedAt,
	).Scan(&msg.ID)

	if err != nil {
		return fmt.Errorf("ошибка создания сообщения: %w", err)
	}

	r.logger.Debug("создано новое сообщение",
		zap.Int64("message_id", msg.ID),
		zap.Int64("user_id", msg.UserID),
		zap.String("role", msg.Role))
	return nil
}

// GetByUserID получает сообщения пользователя с лимитом
func (r *messageRepository) GetByUserID(ctx context.Context, userID int64, limit int) ([]models.UserMessage, error) {
	query := `
		SELECT id, user_id, role, content, created_at
		FROM user_messages 
		WHERE user_id = $1 
		ORDER BY created_at DESC 
		LIMIT $2`

	rows, err := r.db.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения сообщений пользователя: %w", err)
	}
	defer rows.Close()

	var messages []models.UserMessage
	for rows.Next() {
		var msg models.UserMessage
		err := rows.Scan(&msg.ID, &msg.UserID, &msg.Role, &msg.Content, &msg.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования сообщения: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ошибка итерации по сообщениям: %w", err)
	}

	return messages, nil
}

// GetChatHistory получает историю диалога пользователя
func (r *messageRepository) GetChatHistory(ctx context.Context, userID int64, limit int) (*models.ChatHistory, error) {
	// Получаем пользователя
	userRepo := NewUserRepository(r.db, r.logger)
	user, err := userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения пользователя: %w", err)
	}

	// Получаем сообщения
	messages, err := r.GetByUserID(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения сообщений: %w", err)
	}

	// Разворачиваем порядок сообщений (от старых к новым)
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return &models.ChatHistory{
		Messages: messages,
		User:     user,
	}, nil
}

// DeleteByUserID удаляет все сообщения пользователя
func (r *messageRepository) DeleteByUserID(ctx context.Context, userID int64) error {
	query := `DELETE FROM user_messages WHERE user_id = $1`

	result, err := r.db.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("ошибка удаления сообщений пользователя: %w", err)
	}

	r.logger.Info("удалены сообщения пользователя",
		zap.Int64("user_id", userID),
		zap.Int64("deleted_count", result.RowsAffected()))
	return nil
}

// CleanupOldMessages удаляет старые сообщения пользователя, оставляя только последние N
func (r *messageRepository) CleanupOldMessages(ctx context.Context, userID int64, keepCount int) error {
	// Используем оконную функцию для эффективного удаления старых сообщений
	query := `
		DELETE FROM user_messages 
		WHERE id IN (
			SELECT id FROM (
				SELECT id, 
					   ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC) as rn
				FROM user_messages 
				WHERE user_id = $1
			) ranked
			WHERE rn > $2
		)`

	result, err := r.db.Exec(ctx, query, userID, keepCount)
	if err != nil {
		return fmt.Errorf("ошибка очистки старых сообщений: %w", err)
	}

	deletedCount := result.RowsAffected()
	if deletedCount > 0 {
		r.logger.Debug("очищены старые сообщения",
			zap.Int64("user_id", userID),
			zap.Int64("deleted_count", deletedCount),
			zap.Int("keep_count", keepCount))
	}

	return nil
}

// GetMessageCount получает количество сообщений пользователя
func (r *messageRepository) GetMessageCount(ctx context.Context, userID int64) (int, error) {
	query := `SELECT COUNT(*) FROM user_messages WHERE user_id = $1`

	var count int
	err := r.db.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("ошибка получения количества сообщений: %w", err)
	}

	return count, nil
}

// CreateWithCleanup создает новое сообщение с автоматической очисткой старых
func (r *messageRepository) CreateWithCleanup(ctx context.Context, msg *models.UserMessage) error {
	// Начинаем транзакцию для атомарности операций
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer tx.Rollback(ctx)

	// Создаем новое сообщение
	query := `
		INSERT INTO user_messages (user_id, role, content, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id`

	msg.CreatedAt = time.Now()

	err = tx.QueryRow(ctx, query,
		msg.UserID, msg.Role, msg.Content, msg.CreatedAt,
	).Scan(&msg.ID)

	if err != nil {
		return fmt.Errorf("ошибка создания сообщения: %w", err)
	}

	// Проверяем количество сообщений и очищаем при необходимости
	var messageCount int
	countQuery := `SELECT COUNT(*) FROM user_messages WHERE user_id = $1`
	err = tx.QueryRow(ctx, countQuery, msg.UserID).Scan(&messageCount)
	if err != nil {
		return fmt.Errorf("ошибка подсчета сообщений: %w", err)
	}

	// Если превышен лимит - удаляем старые сообщения
	if messageCount > MaxMessagesPerUser {
		cleanupQuery := `
			DELETE FROM user_messages 
			WHERE id IN (
				SELECT id FROM (
					SELECT id, 
						   ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC) as rn
					FROM user_messages 
					WHERE user_id = $1
				) ranked
				WHERE rn > $2
			)`

		result, err := tx.Exec(ctx, cleanupQuery, msg.UserID, MaxMessagesPerUser)
		if err != nil {
			return fmt.Errorf("ошибка автоочистки старых сообщений: %w", err)
		}

		deletedCount := result.RowsAffected()
		if deletedCount > 0 {
			r.logger.Debug("автоочистка старых сообщений",
				zap.Int64("user_id", msg.UserID),
				zap.Int64("deleted_count", deletedCount),
				zap.Int("max_messages", MaxMessagesPerUser))
		}
	}

	// Коммитим транзакцию
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("ошибка коммита транзакции: %w", err)
	}

	r.logger.Debug("создано новое сообщение с автоочисткой",
		zap.Int64("message_id", msg.ID),
		zap.Int64("user_id", msg.UserID),
		zap.String("role", msg.Role))

	return nil
}
