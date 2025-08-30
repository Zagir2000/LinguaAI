package store

import (
	"context"
	"fmt"
	"time"

	"lingua-ai/pkg/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ReferralRepository определяет интерфейс для работы с рефералами
type ReferralRepository interface {
	CreateReferral(ctx context.Context, referral *models.Referral) error
	GetReferralByReferredID(ctx context.Context, referredID int64) (*models.Referral, error)
	GetReferralsByReferrerID(ctx context.Context, referrerID int64) ([]*models.Referral, error)
	UpdateReferralStatus(ctx context.Context, referralID int64, status string, completedAt *time.Time) error
	GetReferralStats(ctx context.Context, userID int64) (*models.ReferralStats, error)
	GetUserByReferralCode(ctx context.Context, referralCode string) (*models.User, error)
	GenerateReferralCode(ctx context.Context) (string, error)
	CountCompletedReferrals(ctx context.Context, userID int64) (int, error)
}

// PostgresReferralRepository реализует ReferralRepository для PostgreSQL
type PostgresReferralRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewReferralRepository создает новый репозиторий рефералов
func NewReferralRepository(db *pgxpool.Pool, logger *zap.Logger) ReferralRepository {
	return &PostgresReferralRepository{
		db:     db,
		logger: logger,
	}
}

// CreateReferral создает новую реферальную связь
func (r *PostgresReferralRepository) CreateReferral(ctx context.Context, referral *models.Referral) error {
	query := `
		INSERT INTO referrals (referrer_id, referred_id, status, created_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id`

	err := r.db.QueryRow(
		ctx, query,
		referral.ReferrerID,
		referral.ReferredID,
		referral.Status,
		referral.CreatedAt,
	).Scan(&referral.ID)

	if err != nil {
		return fmt.Errorf("ошибка создания реферала: %w", err)
	}

	return nil
}

// GetReferralByReferredID получает реферал по ID приглашенного пользователя
func (r *PostgresReferralRepository) GetReferralByReferredID(ctx context.Context, referredID int64) (*models.Referral, error) {
	query := `
		SELECT id, referrer_id, referred_id, status, completed_at, created_at
		FROM referrals 
		WHERE referred_id = $1`

	referral := &models.Referral{}
	err := r.db.QueryRow(ctx, query, referredID).Scan(
		&referral.ID,
		&referral.ReferrerID,
		&referral.ReferredID,
		&referral.Status,
		&referral.CompletedAt,
		&referral.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("реферал не найден")
		}
		return nil, fmt.Errorf("ошибка получения реферала: %w", err)
	}

	return referral, nil
}

// GetReferralsByReferrerID получает все рефералы приглашающего пользователя
func (r *PostgresReferralRepository) GetReferralsByReferrerID(ctx context.Context, referrerID int64) ([]*models.Referral, error) {
	query := `
		SELECT id, referrer_id, referred_id, status, completed_at, created_at
		FROM referrals 
		WHERE referrer_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, query, referrerID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения рефералов: %w", err)
	}
	defer rows.Close()

	var referrals []*models.Referral
	for rows.Next() {
		referral := &models.Referral{}
		err := rows.Scan(
			&referral.ID,
			&referral.ReferrerID,
			&referral.ReferredID,
			&referral.Status,
			&referral.CompletedAt,
			&referral.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("ошибка сканирования реферала: %w", err)
		}
		referrals = append(referrals, referral)
	}

	return referrals, nil
}

// UpdateReferralStatus обновляет статус реферала
func (r *PostgresReferralRepository) UpdateReferralStatus(ctx context.Context, referralID int64, status string, completedAt *time.Time) error {
	query := `
		UPDATE referrals 
		SET status = $1, completed_at = $2
		WHERE id = $3`

	_, err := r.db.Exec(ctx, query, status, completedAt, referralID)
	if err != nil {
		return fmt.Errorf("ошибка обновления статуса реферала: %w", err)
	}

	return nil
}

// GetReferralStats получает статистику рефералов пользователя
func (r *PostgresReferralRepository) GetReferralStats(ctx context.Context, userID int64) (*models.ReferralStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_referrals,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed_referrals,
			COUNT(CASE WHEN status = 'pending' THEN 1 END) as pending_referrals
		FROM referrals 
		WHERE referrer_id = $1`

	stats := &models.ReferralStats{}
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&stats.TotalReferrals,
		&stats.CompletedReferrals,
		&stats.PendingReferrals,
	)

	if err != nil {
		return nil, fmt.Errorf("ошибка получения статистики рефералов: %w", err)
	}

	// Получаем информацию о премиуме
	userQuery := `
		SELECT referral_count
		FROM users 
		WHERE id = $1`

	var referralCount int
	err = r.db.QueryRow(ctx, userQuery, userID).Scan(&referralCount)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения информации о рефералах: %w", err)
	}

	// Вычисляем сколько рефералов нужно до премиума
	if referralCount >= 10 {
		stats.ReferralsToPremium = 0
	} else {
		stats.ReferralsToPremium = 10 - referralCount
		if stats.ReferralsToPremium < 0 {
			stats.ReferralsToPremium = 0
		}
	}

	return stats, nil
}

// GetUserByReferralCode получает пользователя по реферальному коду
func (r *PostgresReferralRepository) GetUserByReferralCode(ctx context.Context, referralCode string) (*models.User, error) {
	query := `
		SELECT id, telegram_id, username, first_name, last_name, level, xp, 
		       last_seen, created_at, updated_at, current_state, study_streak,
		       last_study_date, is_premium, premium_expires_at, messages_count,
		       max_messages, messages_reset_date, last_test_date, referral_code,
		       referral_count, referred_by
		FROM users 
		WHERE referral_code = $1`

	user := &models.User{}
	err := r.db.QueryRow(ctx, query, referralCode).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName,
		&user.Level, &user.XP, &user.LastSeen, &user.CreatedAt, &user.UpdatedAt,
		&user.CurrentState, &user.StudyStreak, &user.LastStudyDate, &user.IsPremium,
		&user.PremiumExpiresAt, &user.MessagesCount, &user.MaxMessages,
		&user.MessagesResetDate, &user.LastTestDate, &user.ReferralCode,
		&user.ReferralCount, &user.ReferredBy,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("пользователь с таким реферальным кодом не найден")
		}
		return nil, fmt.Errorf("ошибка получения пользователя: %w", err)
	}

	return user, nil
}

// GenerateReferralCode генерирует уникальный реферальный код
func (r *PostgresReferralRepository) GenerateReferralCode(ctx context.Context) (string, error) {
	query := `SELECT generate_referral_code()`

	var code string
	err := r.db.QueryRow(ctx, query).Scan(&code)
	if err != nil {
		return "", fmt.Errorf("ошибка генерации реферального кода: %w", err)
	}

	return code, nil
}

// CountCompletedReferrals подсчитывает количество завершенных рефералов пользователя
func (r *PostgresReferralRepository) CountCompletedReferrals(ctx context.Context, userID int64) (int, error) {
	query := `
		SELECT COUNT(*) 
		FROM referrals 
		WHERE referrer_id = $1 AND status = 'completed'`

	var count int
	err := r.db.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("ошибка подсчета рефералов: %w", err)
	}

	return count, nil
}
