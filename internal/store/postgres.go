package store

import (
	"context"
	"fmt"
	"time"

	"lingua-ai/internal/config"
	"lingua-ai/pkg/models"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Store представляет интерфейс для работы с базой данных
type Store interface {
	User() UserRepository
	Message() MessageRepository
	Flashcard() FlashcardRepository
	Referral() ReferralRepository
	Payment() PaymentRepository
	DB() *pgxpool.Pool
	Close() error
}

// store реализует интерфейс Store
type store struct {
	db        *pgxpool.Pool
	logger    *zap.Logger
	user      UserRepository
	msg       MessageRepository
	flashcard FlashcardRepository
	referral  ReferralRepository
	payment   PaymentRepository
}

// UserRepository интерфейс для работы с пользователями
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id int64) (*models.User, error)
	GetByTelegramID(ctx context.Context, telegramID int64) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	UpdateState(ctx context.Context, userID int64, state string) error
	AddXP(ctx context.Context, userID int64, xp int) error
	UpdateLastSeen(ctx context.Context, userID int64) error
	UpdateStudyActivity(ctx context.Context, userID int64) error
	GetStats(ctx context.Context, userID int64) (*models.UserStats, error)
	GetTopUsersByStreak(ctx context.Context, limit int) ([]*models.User, error)
	GetAll(ctx context.Context) ([]*models.User, error)
	GetInactiveUsers(ctx context.Context, inactiveDuration time.Duration) ([]*models.User, error)
	IncrementMessagesCount(ctx context.Context, userID int64) error
}

// MessageRepository интерфейс для работы с сообщениями
type MessageRepository interface {
	Create(ctx context.Context, msg *models.UserMessage) error
	CreateWithCleanup(ctx context.Context, msg *models.UserMessage) error
	GetByUserID(ctx context.Context, userID int64, limit int) ([]models.UserMessage, error)
	GetChatHistory(ctx context.Context, userID int64, limit int) (*models.ChatHistory, error)
	GetMessageCount(ctx context.Context, userID int64) (int, error)
	CleanupOldMessages(ctx context.Context, userID int64, keepCount int) error
	DeleteByUserID(ctx context.Context, userID int64) error
}

// PaymentRepository интерфейс для работы с платежами
type PaymentRepository interface {
	Create(ctx context.Context, payment *models.Payment) error
	GetByPaymentID(ctx context.Context, paymentID string) (*models.Payment, error)
	Update(ctx context.Context, payment *models.Payment) error
}

// NewStore создает новое подключение к базе данных
func NewStore(cfg *config.Config, logger *zap.Logger) (Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Создание пула подключений
	poolConfig, err := pgxpool.ParseConfig(cfg.Database.GetDSN())
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга DSN: %w", err)
	}

	// Настройка пула
	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	// Создание пула
	db, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к базе данных: %w", err)
	}

	// Проверка подключения
	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ошибка проверки подключения к базе данных: %w", err)
	}

	logger.Info("успешное подключение к базе данных PostgreSQL")

	s := &store{
		db:     db,
		logger: logger,
	}

	// Инициализация репозиториев
	s.user = NewUserRepository(db, logger)
	s.msg = NewMessageRepository(db, logger)
	s.flashcard = NewFlashcardRepository(db, logger)
	s.referral = NewReferralRepository(db, logger)
	s.payment = NewPaymentRepository(db, logger)

	return s, nil
}

// User возвращает репозиторий пользователей
func (s *store) User() UserRepository {
	return s.user
}

// Message возвращает репозиторий сообщений
func (s *store) Message() MessageRepository {
	return s.msg
}

// Flashcard возвращает репозиторий карточек
func (s *store) Flashcard() FlashcardRepository {
	return s.flashcard
}

// Referral возвращает репозиторий рефералов
func (s *store) Referral() ReferralRepository {
	return s.referral
}

// Payment возвращает репозиторий платежей
func (s *store) Payment() PaymentRepository {
	return s.payment
}

// DB возвращает подключение к базе данных
func (s *store) DB() *pgxpool.Pool {
	return s.db
}

// Close закрывает подключение к базе данных
func (s *store) Close() error {
	s.logger.Info("закрытие подключения к базе данных")
	s.db.Close()
	return nil
}

// userRepository реализует UserRepository
type userRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewUserRepository создает новый репозиторий пользователей
func NewUserRepository(db *pgxpool.Pool, logger *zap.Logger) UserRepository {
	return &userRepository{
		db:     db,
		logger: logger,
	}
}

// Create создает нового пользователя
func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (telegram_id, username, first_name, last_name, level, xp, study_streak, last_study_date, current_state, last_seen, created_at, updated_at,
		                  is_premium, premium_expires_at, messages_count, max_messages, messages_reset_date, last_test_date,
		                  referral_count, referred_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		RETURNING id`

	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	user.LastSeen = now
	user.LastStudyDate = now
	user.MessagesResetDate = now.Truncate(24 * time.Hour) // Начало текущего дня

	// Устанавливаем значения по умолчанию
	if user.CurrentState == "" {
		user.CurrentState = "idle" // Статус по умолчанию
	}
	if user.MaxMessages == 0 {
		user.MaxMessages = 7 // Новый лимит по умолчанию для бесплатных пользователей
	}

	err := r.db.QueryRow(ctx, query,
		user.TelegramID, user.Username, user.FirstName, user.LastName,
		user.Level, user.XP, user.StudyStreak, user.LastStudyDate, user.CurrentState, user.LastSeen, user.CreatedAt, user.UpdatedAt,
		user.IsPremium, user.PremiumExpiresAt, user.MessagesCount, user.MaxMessages, user.MessagesResetDate, user.LastTestDate,
		user.ReferralCount, user.ReferredBy,
	).Scan(&user.ID)

	if err != nil {
		return fmt.Errorf("ошибка создания пользователя: %w", err)
	}

	r.logger.Info("пользователь создан",
		zap.Int64("user_id", user.ID),
		zap.Int64("telegram_id", user.TelegramID),
		zap.String("username", user.Username),
		zap.String("current_state", user.CurrentState))

	return nil
}

// GetByID получает пользователя по ID
func (r *userRepository) GetByID(ctx context.Context, id int64) (*models.User, error) {
	query := `
		SELECT id, telegram_id, username, first_name, last_name, level, xp, study_streak, last_study_date, current_state, last_seen, created_at, updated_at,
		       is_premium, premium_expires_at, messages_count, max_messages, messages_reset_date, last_test_date,
		       referral_code, referral_count, referred_by
		FROM users WHERE id = $1`

	user := &models.User{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName,
		&user.Level, &user.XP, &user.StudyStreak, &user.LastStudyDate, &user.CurrentState, &user.LastSeen, &user.CreatedAt, &user.UpdatedAt,
		&user.IsPremium, &user.PremiumExpiresAt, &user.MessagesCount, &user.MaxMessages, &user.MessagesResetDate, &user.LastTestDate,
		&user.ReferralCode, &user.ReferralCount, &user.ReferredBy,
	)

	if err != nil {
		return nil, fmt.Errorf("ошибка получения пользователя по ID: %w", err)
	}

	return user, nil
}

// GetByTelegramID получает пользователя по Telegram ID
func (r *userRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*models.User, error) {
	query := `
		SELECT id, telegram_id, username, first_name, last_name, level, xp, study_streak, last_study_date, current_state, last_seen, created_at, updated_at,
		       is_premium, premium_expires_at, messages_count, max_messages, messages_reset_date, last_test_date,
		       referral_code, referral_count, referred_by
		FROM users WHERE telegram_id = $1`

	user := &models.User{}
	err := r.db.QueryRow(ctx, query, telegramID).Scan(
		&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName,
		&user.Level, &user.XP, &user.StudyStreak, &user.LastStudyDate, &user.CurrentState, &user.LastSeen, &user.CreatedAt, &user.UpdatedAt,
		&user.IsPremium, &user.PremiumExpiresAt, &user.MessagesCount, &user.MaxMessages, &user.MessagesResetDate, &user.LastTestDate,
		&user.ReferralCode, &user.ReferralCount, &user.ReferredBy,
	)

	if err != nil {
		return nil, fmt.Errorf("ошибка получения пользователя по Telegram ID: %w", err)
	}

	return user, nil
}

// Update обновляет пользователя
func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	query := `
		UPDATE users 
		SET username = $2, first_name = $3, last_name = $4, level = $5, xp = $6, current_state = $7, last_seen = $8, updated_at = $9,
		    is_premium = $10, premium_expires_at = $11, messages_count = $12, max_messages = $13, messages_reset_date = $14, last_test_date = $15,
		    referral_code = $16, referral_count = $17, referred_by = $18
		WHERE id = $1`

	user.UpdatedAt = time.Now()

	result, err := r.db.Exec(ctx, query,
		user.ID, user.Username, user.FirstName, user.LastName,
		user.Level, user.XP, user.CurrentState, user.LastSeen, user.UpdatedAt,
		user.IsPremium, user.PremiumExpiresAt, user.MessagesCount, user.MaxMessages, user.MessagesResetDate, user.LastTestDate,
		user.ReferralCode, user.ReferralCount, user.ReferredBy,
	)

	if err != nil {
		return fmt.Errorf("ошибка обновления пользователя: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("пользователь с ID %d не найден", user.ID)
	}

	r.logger.Info("пользователь обновлен", zap.Int64("user_id", user.ID))
	return nil
}

// IncrementMessagesCount увеличивает счетчик сообщений пользователя
func (r *userRepository) IncrementMessagesCount(ctx context.Context, userID int64) error {
	query := `UPDATE users SET messages_count = messages_count + 1, updated_at = $2 WHERE id = $1`

	now := time.Now()

	r.logger.Info("увеличиваем счетчик сообщений",
		zap.Int64("user_id", userID),
		zap.String("query", query),
		zap.Time("now", now))

	result, err := r.db.Exec(ctx, query, userID, now)

	if err != nil {
		r.logger.Error("ошибка увеличения счетчика сообщений",
			zap.Error(err),
			zap.Int64("user_id", userID))
		return fmt.Errorf("ошибка увеличения счетчика сообщений: %w", err)
	}

	rowsAffected := result.RowsAffected()
	r.logger.Info("результат обновления счетчика сообщений",
		zap.Int64("user_id", userID),
		zap.Int64("rows_affected", rowsAffected))

	if rowsAffected == 0 {
		return fmt.Errorf("пользователь с ID %d не найден", userID)
	}

	r.logger.Info("счетчик сообщений увеличен", zap.Int64("user_id", userID))
	return nil
}

// UpdateLastSeen обновляет время последнего посещения
func (r *userRepository) UpdateLastSeen(ctx context.Context, userID int64) error {
	query := `UPDATE users SET last_seen = $2, updated_at = $3 WHERE id = $1`

	now := time.Now()
	result, err := r.db.Exec(ctx, query, userID, now, now)

	if err != nil {
		return fmt.Errorf("ошибка обновления времени последнего посещения: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("пользователь с ID %d не найден", userID)
	}

	return nil
}

// GetStats получает статистику пользователя
func (r *userRepository) GetStats(ctx context.Context, userID int64) (*models.UserStats, error) {
	query := `
		SELECT 
			u.id as user_id,
			u.xp as total_xp,
			u.study_streak,
			u.last_study_date
		FROM users u
		WHERE u.id = $1`

	stats := &models.UserStats{}
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&stats.UserID, &stats.TotalXP, &stats.StudyStreak, &stats.LastStudyDate,
	)

	if err != nil {
		return nil, fmt.Errorf("ошибка получения статистики пользователя: %w", err)
	}

	return stats, nil
}

// UpdateState обновляет состояние пользователя
func (r *userRepository) UpdateState(ctx context.Context, userID int64, state string) error {
	query := `UPDATE users SET current_state = $2, updated_at = $3 WHERE id = $1`

	now := time.Now()
	result, err := r.db.Exec(ctx, query, userID, state, now)

	if err != nil {
		return fmt.Errorf("ошибка обновления состояния пользователя: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("пользователь с ID %d не найден", userID)
	}

	r.logger.Info("состояние пользователя обновлено",
		zap.Int64("user_id", userID),
		zap.String("state", state))
	return nil
}

// AddXP добавляет опыт пользователю
func (r *userRepository) AddXP(ctx context.Context, userID int64, xp int) error {
	query := `UPDATE users SET xp = xp + $2, updated_at = $3 WHERE id = $1`

	now := time.Now()
	result, err := r.db.Exec(ctx, query, userID, xp, now)

	if err != nil {
		return fmt.Errorf("ошибка добавления XP: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("пользователь с ID %d не найден", userID)
	}

	r.logger.Info("XP добавлен пользователю",
		zap.Int64("user_id", userID),
		zap.Int("xp_added", xp))
	return nil
}

// UpdateStudyActivity обновляет активность обучения пользователя
func (r *userRepository) UpdateStudyActivity(ctx context.Context, userID int64) error {
	// Получаем текущего пользователя
	user, err := r.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("ошибка получения пользователя: %w", err)
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	lastStudyDay := time.Date(user.LastStudyDate.Year(), user.LastStudyDate.Month(), user.LastStudyDate.Day(), 0, 0, 0, 0, user.LastStudyDate.Location())

	var newStreak int
	if today.Equal(lastStudyDay) {
		// Пользователь уже занимался сегодня, streak не меняется
		newStreak = user.StudyStreak
	} else if today.Sub(lastStudyDay) <= 24*time.Hour {
		// Занимался вчера или сегодня (в пределах 24 часов)
		newStreak = user.StudyStreak + 1
	} else if today.Sub(lastStudyDay) <= 48*time.Hour {
		// Пропустил 1 день - даем шанс, сохраняем текущий streak
		newStreak = user.StudyStreak
	} else {
		// Пропустил больше 1 дня - сбрасываем streak
		newStreak = 1
	}

	// Обновляем пользователя
	query := `UPDATE users SET study_streak = $2, last_study_date = $3, last_seen = $4, updated_at = $5 WHERE id = $1`
	result, err := r.db.Exec(ctx, query, userID, newStreak, now, now, now)
	if err != nil {
		return fmt.Errorf("ошибка обновления активности обучения: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("пользователь с ID %d не найден", userID)
	}

	r.logger.Info("активность обучения обновлена",
		zap.Int64("user_id", userID),
		zap.Int("old_streak", user.StudyStreak),
		zap.Int("new_streak", newStreak))

	return nil
}

// GetTopUsersByStreak получает топ пользователей по XP и study streak
func (r *userRepository) GetTopUsersByStreak(ctx context.Context, limit int) ([]*models.User, error) {
	query := `
		SELECT id, telegram_id, username, first_name, last_name, level, xp, study_streak, last_study_date, current_state, last_seen, created_at, updated_at,
		       is_premium, premium_expires_at, messages_count, max_messages, messages_reset_date, last_test_date
		FROM users
		ORDER BY xp DESC, study_streak DESC, last_study_date DESC
		LIMIT $1
	`

	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		r.logger.Error("ошибка получения топ пользователей по streak", zap.Error(err))
		return nil, fmt.Errorf("ошибка получения топ пользователей по streak: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		err := rows.Scan(
			&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName,
			&user.Level, &user.XP, &user.StudyStreak, &user.LastStudyDate, &user.CurrentState,
			&user.LastSeen, &user.CreatedAt, &user.UpdatedAt,
			&user.IsPremium, &user.PremiumExpiresAt, &user.MessagesCount, &user.MaxMessages, &user.MessagesResetDate, &user.LastTestDate,
		)
		if err != nil {
			r.logger.Error("ошибка сканирования пользователя", zap.Error(err))
			continue
		}
		users = append(users, user)
	}

	return users, nil
}

// GetInactiveUsers получает пользователей, неактивных более указанного времени
func (r *userRepository) GetInactiveUsers(ctx context.Context, inactiveDuration time.Duration) ([]*models.User, error) {
	cutoffTime := time.Now().Add(-inactiveDuration)

	query := `
		SELECT id, telegram_id, username, first_name, last_name, level, xp, study_streak, last_study_date, current_state, last_seen, created_at, updated_at,
		       is_premium, premium_expires_at, messages_count, max_messages, messages_reset_date, last_test_date
		FROM users 
		WHERE last_seen < $1
		ORDER BY last_seen ASC
	`

	rows, err := r.db.Query(ctx, query, cutoffTime)
	if err != nil {
		r.logger.Error("ошибка получения неактивных пользователей", zap.Error(err))
		return nil, fmt.Errorf("ошибка получения неактивных пользователей: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		err := rows.Scan(
			&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName,
			&user.Level, &user.XP, &user.StudyStreak, &user.LastStudyDate, &user.CurrentState,
			&user.LastSeen, &user.CreatedAt, &user.UpdatedAt,
			&user.IsPremium, &user.PremiumExpiresAt, &user.MessagesCount, &user.MaxMessages, &user.MessagesResetDate, &user.LastTestDate,
		)
		if err != nil {
			r.logger.Error("ошибка сканирования неактивного пользователя", zap.Error(err))
			continue
		}
		users = append(users, user)
	}

	r.logger.Info("найдено неактивных пользователей",
		zap.Int("count", len(users)),
		zap.Duration("inactive_duration", inactiveDuration),
		zap.Time("cutoff_time", cutoffTime))

	return users, nil
}

// GetAll получает всех пользователей
func (r *userRepository) GetAll(ctx context.Context) ([]*models.User, error) {
	query := `
		SELECT id, telegram_id, username, first_name, last_name, level, xp, study_streak, last_study_date, current_state, last_seen, created_at, updated_at,
		       is_premium, premium_expires_at, messages_count, max_messages, messages_reset_date, last_test_date
		FROM users
		ORDER BY xp DESC, last_study_date DESC
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		r.logger.Error("ошибка получения всех пользователей", zap.Error(err))
		return nil, fmt.Errorf("ошибка получения всех пользователей: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		err := rows.Scan(
			&user.ID, &user.TelegramID, &user.Username, &user.FirstName, &user.LastName,
			&user.Level, &user.XP, &user.StudyStreak, &user.LastStudyDate, &user.CurrentState,
			&user.LastSeen, &user.CreatedAt, &user.UpdatedAt,
			&user.IsPremium, &user.PremiumExpiresAt, &user.MessagesCount, &user.MaxMessages, &user.MessagesResetDate, &user.LastTestDate,
		)
		if err != nil {
			r.logger.Error("ошибка сканирования пользователя", zap.Error(err))
			continue
		}
		users = append(users, user)
	}

	return users, nil
}
