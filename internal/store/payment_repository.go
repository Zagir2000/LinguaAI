package store

import (
	"context"
	"fmt"

	"lingua-ai/pkg/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// PostgresPaymentRepository реализует PaymentRepository для PostgreSQL
type PostgresPaymentRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewPaymentRepository создает новый репозиторий платежей
func NewPaymentRepository(db *pgxpool.Pool, logger *zap.Logger) PaymentRepository {
	return &PostgresPaymentRepository{
		db:     db,
		logger: logger,
	}
}

// Create создает новый платеж
func (r *PostgresPaymentRepository) Create(ctx context.Context, payment *models.Payment) error {
	query := `
		INSERT INTO payments (
			user_id, amount, currency, payment_id, status, 
			premium_duration_days, created_at, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`

	err := r.db.QueryRow(
		ctx, query,
		payment.UserID,
		payment.Amount,
		payment.Currency,
		payment.PaymentID,
		payment.Status,
		payment.PremiumDurationDays,
		payment.CreatedAt,
		payment.Metadata,
	).Scan(&payment.ID)

	if err != nil {
		return fmt.Errorf("ошибка создания платежа: %w", err)
	}

	r.logger.Info("платеж создан в БД",
		zap.Int64("payment_id", payment.ID),
		zap.Int64("user_id", payment.UserID),
		zap.String("yukassa_payment_id", payment.PaymentID))

	return nil
}

// GetByPaymentID получает платеж по ID от ЮKassa
func (r *PostgresPaymentRepository) GetByPaymentID(ctx context.Context, paymentID string) (*models.Payment, error) {
	query := `
		SELECT id, user_id, amount, currency, payment_id, status, 
		       premium_duration_days, created_at, completed_at, metadata
		FROM payments 
		WHERE payment_id = $1`

	payment := &models.Payment{}
	err := r.db.QueryRow(ctx, query, paymentID).Scan(
		&payment.ID,
		&payment.UserID,
		&payment.Amount,
		&payment.Currency,
		&payment.PaymentID,
		&payment.Status,
		&payment.PremiumDurationDays,
		&payment.CreatedAt,
		&payment.CompletedAt,
		&payment.Metadata,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("платеж не найден")
		}
		return nil, fmt.Errorf("ошибка получения платежа: %w", err)
	}

	return payment, nil
}

// Update обновляет платеж
func (r *PostgresPaymentRepository) Update(ctx context.Context, payment *models.Payment) error {
	query := `
		UPDATE payments 
		SET status = $1, completed_at = $2, metadata = $3
		WHERE id = $4`

	_, err := r.db.Exec(
		ctx, query,
		payment.Status,
		payment.CompletedAt,
		payment.Metadata,
		payment.ID,
	)

	if err != nil {
		return fmt.Errorf("ошибка обновления платежа: %w", err)
	}

	r.logger.Info("платеж обновлен в БД",
		zap.Int64("payment_id", payment.ID),
		zap.String("status", payment.Status))

	return nil
}
