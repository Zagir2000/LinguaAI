package store

import (
	"context"
	"fmt"

	"lingua-ai/pkg/models"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// FlashcardRepository интерфейс для работы со словарными карточками
type FlashcardRepository interface {
	// Flashcards
	GetFlashcardByID(ctx context.Context, id int64) (*models.Flashcard, error)
	GetFlashcardsByLevel(ctx context.Context, level string, limit int) ([]*models.Flashcard, error)
	GetFlashcardsByCategory(ctx context.Context, category string, limit int) ([]*models.Flashcard, error)
	GetRandomFlashcards(ctx context.Context, level string, limit int) ([]*models.Flashcard, error)

	// User Flashcards
	GetUserFlashcard(ctx context.Context, userID, flashcardID int64) (*models.UserFlashcard, error)
	CreateUserFlashcard(ctx context.Context, userFlashcard *models.UserFlashcard) error
	UpdateUserFlashcard(ctx context.Context, userFlashcard *models.UserFlashcard) error
	GetUserFlashcardsForReview(ctx context.Context, userID int64, limit int) ([]*models.UserFlashcard, error)
	GetUserFlashcardStats(ctx context.Context, userID int64) (map[string]interface{}, error)
	GetLearnedWordsCount(ctx context.Context, userID int64) (int, error)

	// Spaced Repetition
	GetCardsToReview(ctx context.Context, userID int64) ([]*models.UserFlashcard, error)
	GetNewCardsForUser(ctx context.Context, userID int64, level string, limit int) ([]*models.Flashcard, error)
	GetNextCardToReview(ctx context.Context, userID int64) (*models.UserFlashcard, error)
}

// flashcardRepository реализация FlashcardRepository
type flashcardRepository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewFlashcardRepository создает новый репозиторий для карточек
func NewFlashcardRepository(db *pgxpool.Pool, logger *zap.Logger) FlashcardRepository {
	return &flashcardRepository{
		db:     db,
		logger: logger,
	}
}

// GetFlashcardByID получает карточку по ID
func (r *flashcardRepository) GetFlashcardByID(ctx context.Context, id int64) (*models.Flashcard, error) {
	query := `
		SELECT id, word, translation, example, level, category, created_at
		FROM flashcards 
		WHERE id = $1`

	flashcard := &models.Flashcard{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&flashcard.ID, &flashcard.Word, &flashcard.Translation,
		&flashcard.Example, &flashcard.Level, &flashcard.Category, &flashcard.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("ошибка получения карточки: %w", err)
	}

	return flashcard, nil
}

// GetFlashcardsByLevel получает карточки по уровню
func (r *flashcardRepository) GetFlashcardsByLevel(ctx context.Context, level string, limit int) ([]*models.Flashcard, error) {
	query := `
		SELECT id, word, translation, example, level, category, created_at
		FROM flashcards 
		WHERE level = $1
		ORDER BY RANDOM()
		LIMIT $2`

	rows, err := r.db.Query(ctx, query, level, limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения карточек по уровню: %w", err)
	}
	defer rows.Close()

	var flashcards []*models.Flashcard
	for rows.Next() {
		flashcard := &models.Flashcard{}
		err := rows.Scan(
			&flashcard.ID, &flashcard.Word, &flashcard.Translation,
			&flashcard.Example, &flashcard.Level, &flashcard.Category, &flashcard.CreatedAt,
		)
		if err != nil {
			r.logger.Error("ошибка сканирования карточки", zap.Error(err))
			continue
		}
		flashcards = append(flashcards, flashcard)
	}

	return flashcards, nil
}

// GetFlashcardsByCategory получает карточки по категории
func (r *flashcardRepository) GetFlashcardsByCategory(ctx context.Context, category string, limit int) ([]*models.Flashcard, error) {
	query := `
		SELECT id, word, translation, example, level, category, created_at
		FROM flashcards 
		WHERE category = $1
		ORDER BY RANDOM()
		LIMIT $2`

	rows, err := r.db.Query(ctx, query, category, limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения карточек по категории: %w", err)
	}
	defer rows.Close()

	var flashcards []*models.Flashcard
	for rows.Next() {
		flashcard := &models.Flashcard{}
		err := rows.Scan(
			&flashcard.ID, &flashcard.Word, &flashcard.Translation,
			&flashcard.Example, &flashcard.Level, &flashcard.Category, &flashcard.CreatedAt,
		)
		if err != nil {
			r.logger.Error("ошибка сканирования карточки", zap.Error(err))
			continue
		}
		flashcards = append(flashcards, flashcard)
	}

	return flashcards, nil
}

// GetRandomFlashcards получает случайные карточки
func (r *flashcardRepository) GetRandomFlashcards(ctx context.Context, level string, limit int) ([]*models.Flashcard, error) {
	query := `
		SELECT id, word, translation, example, level, category, created_at
		FROM flashcards 
		WHERE level = $1
		ORDER BY RANDOM()
		LIMIT $2`

	rows, err := r.db.Query(ctx, query, level, limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения случайных карточек: %w", err)
	}
	defer rows.Close()

	var flashcards []*models.Flashcard
	for rows.Next() {
		flashcard := &models.Flashcard{}
		err := rows.Scan(
			&flashcard.ID, &flashcard.Word, &flashcard.Translation,
			&flashcard.Example, &flashcard.Level, &flashcard.Category, &flashcard.CreatedAt,
		)
		if err != nil {
			r.logger.Error("ошибка сканирования карточки", zap.Error(err))
			continue
		}
		flashcards = append(flashcards, flashcard)
	}

	return flashcards, nil
}

// GetUserFlashcard получает прогресс пользователя по карточке
func (r *flashcardRepository) GetUserFlashcard(ctx context.Context, userID, flashcardID int64) (*models.UserFlashcard, error) {
	query := `
		SELECT uf.id, uf.user_id, uf.flashcard_id, uf.difficulty, uf.review_count, 
		       uf.correct_count, uf.last_reviewed_at, uf.next_review_at, uf.is_learned, uf.created_at,
		       f.id, f.word, f.translation, f.example, f.level, f.category, f.created_at
		FROM user_flashcards uf
		JOIN flashcards f ON uf.flashcard_id = f.id
		WHERE uf.user_id = $1 AND uf.flashcard_id = $2`

	userFlashcard := &models.UserFlashcard{
		Flashcard: &models.Flashcard{},
	}

	err := r.db.QueryRow(ctx, query, userID, flashcardID).Scan(
		&userFlashcard.ID, &userFlashcard.UserID, &userFlashcard.FlashcardID,
		&userFlashcard.Difficulty, &userFlashcard.ReviewCount, &userFlashcard.CorrectCount,
		&userFlashcard.LastReviewedAt, &userFlashcard.NextReviewAt, &userFlashcard.IsLearned, &userFlashcard.CreatedAt,
		&userFlashcard.Flashcard.ID, &userFlashcard.Flashcard.Word, &userFlashcard.Flashcard.Translation,
		&userFlashcard.Flashcard.Example, &userFlashcard.Flashcard.Level, &userFlashcard.Flashcard.Category, &userFlashcard.Flashcard.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("ошибка получения пользовательской карточки: %w", err)
	}

	return userFlashcard, nil
}

// CreateUserFlashcard создает новую запись прогресса пользователя
func (r *flashcardRepository) CreateUserFlashcard(ctx context.Context, userFlashcard *models.UserFlashcard) error {
	query := `
		INSERT INTO user_flashcards (user_id, flashcard_id, difficulty, review_count, 
		                           correct_count, next_review_at, is_learned)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`

	err := r.db.QueryRow(ctx, query,
		userFlashcard.UserID, userFlashcard.FlashcardID, userFlashcard.Difficulty,
		userFlashcard.ReviewCount, userFlashcard.CorrectCount, userFlashcard.NextReviewAt,
		userFlashcard.IsLearned,
	).Scan(&userFlashcard.ID, &userFlashcard.CreatedAt)

	if err != nil {
		return fmt.Errorf("ошибка создания пользовательской карточки: %w", err)
	}

	return nil
}

// UpdateUserFlashcard обновляет прогресс пользователя по карточке
func (r *flashcardRepository) UpdateUserFlashcard(ctx context.Context, userFlashcard *models.UserFlashcard) error {
	query := `
		UPDATE user_flashcards 
		SET difficulty = $3, review_count = $4, correct_count = $5, 
		    last_reviewed_at = $6, next_review_at = $7, is_learned = $8
		WHERE user_id = $1 AND flashcard_id = $2`

	_, err := r.db.Exec(ctx, query,
		userFlashcard.UserID, userFlashcard.FlashcardID, userFlashcard.Difficulty,
		userFlashcard.ReviewCount, userFlashcard.CorrectCount, userFlashcard.LastReviewedAt,
		userFlashcard.NextReviewAt, userFlashcard.IsLearned,
	)

	if err != nil {
		return fmt.Errorf("ошибка обновления пользовательской карточки: %w", err)
	}

	return nil
}

// GetUserFlashcardsForReview получает карточки для повторения
func (r *flashcardRepository) GetUserFlashcardsForReview(ctx context.Context, userID int64, limit int) ([]*models.UserFlashcard, error) {
	query := `
		SELECT uf.id, uf.user_id, uf.flashcard_id, uf.difficulty, uf.review_count, 
		       uf.correct_count, uf.last_reviewed_at, uf.next_review_at, uf.is_learned, uf.created_at,
		       f.id, f.word, f.translation, f.example, f.level, f.category, f.created_at
		FROM user_flashcards uf
		JOIN flashcards f ON uf.flashcard_id = f.id
		WHERE uf.user_id = $1 AND uf.next_review_at <= CURRENT_TIMESTAMP AND uf.is_learned = FALSE
		ORDER BY uf.next_review_at ASC
		LIMIT $2`

	rows, err := r.db.Query(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения карточек для повторения: %w", err)
	}
	defer rows.Close()

	var userFlashcards []*models.UserFlashcard
	for rows.Next() {
		userFlashcard := &models.UserFlashcard{
			Flashcard: &models.Flashcard{},
		}

		err := rows.Scan(
			&userFlashcard.ID, &userFlashcard.UserID, &userFlashcard.FlashcardID,
			&userFlashcard.Difficulty, &userFlashcard.ReviewCount, &userFlashcard.CorrectCount,
			&userFlashcard.LastReviewedAt, &userFlashcard.NextReviewAt, &userFlashcard.IsLearned, &userFlashcard.CreatedAt,
			&userFlashcard.Flashcard.ID, &userFlashcard.Flashcard.Word, &userFlashcard.Flashcard.Translation,
			&userFlashcard.Flashcard.Example, &userFlashcard.Flashcard.Level, &userFlashcard.Flashcard.Category, &userFlashcard.Flashcard.CreatedAt,
		)
		if err != nil {
			r.logger.Error("ошибка сканирования пользовательской карточки", zap.Error(err))
			continue
		}
		userFlashcards = append(userFlashcards, userFlashcard)
	}

	return userFlashcards, nil
}

// GetUserFlashcardStats получает статистику пользователя по карточкам
func (r *flashcardRepository) GetUserFlashcardStats(ctx context.Context, userID int64) (map[string]interface{}, error) {
	query := `
		SELECT 
			COUNT(*) as total_cards,
			COUNT(CASE WHEN is_learned = TRUE THEN 1 END) as learned_cards,
			COUNT(CASE WHEN next_review_at <= CURRENT_TIMESTAMP AND is_learned = FALSE THEN 1 END) as cards_to_review,
			COALESCE(AVG(CASE WHEN review_count > 0 THEN (correct_count::FLOAT / review_count::FLOAT) * 100 END), 0) as accuracy_percentage
		FROM user_flashcards 
		WHERE user_id = $1`

	var totalCards, learnedCards, cardsToReview int
	var accuracyPercentage float64

	err := r.db.QueryRow(ctx, query, userID).Scan(
		&totalCards, &learnedCards, &cardsToReview, &accuracyPercentage,
	)

	if err != nil {
		return nil, fmt.Errorf("ошибка получения статистики карточек: %w", err)
	}

	stats := map[string]interface{}{
		"total_cards":         totalCards,
		"learned_cards":       learnedCards,
		"cards_to_review":     cardsToReview,
		"accuracy_percentage": accuracyPercentage,
	}

	return stats, nil
}

// GetLearnedWordsCount получает количество выученных слов
func (r *flashcardRepository) GetLearnedWordsCount(ctx context.Context, userID int64) (int, error) {
	query := `SELECT COUNT(*) FROM user_flashcards WHERE user_id = $1 AND is_learned = TRUE`

	var count int
	err := r.db.QueryRow(ctx, query, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("ошибка получения количества выученных слов: %w", err)
	}

	return count, nil
}

// GetCardsToReview получает карточки, которые нужно повторить
func (r *flashcardRepository) GetCardsToReview(ctx context.Context, userID int64) ([]*models.UserFlashcard, error) {
	return r.GetUserFlashcardsForReview(ctx, userID, 50) // Максимум 50 карточек за раз
}

// GetNewCardsForUser получает новые карточки для пользователя
func (r *flashcardRepository) GetNewCardsForUser(ctx context.Context, userID int64, level string, limit int) ([]*models.Flashcard, error) {
	r.logger.Info("получение новых карточек для пользователя",
		zap.Int64("user_id", userID),
		zap.String("level", level),
		zap.Int("limit", limit))

	// Сначала проверим общее количество карточек для отладки
	var totalCards int
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM flashcards WHERE level = $1", level).Scan(&totalCards)
	if err != nil {
		r.logger.Error("ошибка подсчета карточек", zap.Error(err))
	} else {
		r.logger.Info("общее количество карточек в базе",
			zap.String("level", level),
			zap.Int("total_cards", totalCards))
	}

	// Проверим количество уже изученных карточек пользователем
	var userCards int
	err = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM user_flashcards uf JOIN flashcards f ON f.id = uf.flashcard_id WHERE uf.user_id = $1 AND f.level = $2", userID, level).Scan(&userCards)
	if err != nil {
		r.logger.Error("ошибка подсчета пользовательских карточек", zap.Error(err))
	} else {
		r.logger.Info("карточки пользователя",
			zap.Int64("user_id", userID),
			zap.String("level", level),
			zap.Int("user_cards", userCards))
	}

	query := `
		SELECT f.id, f.word, f.translation, f.example, f.level, f.category, f.created_at
		FROM flashcards f
		LEFT JOIN user_flashcards uf ON f.id = uf.flashcard_id AND uf.user_id = $1
		WHERE uf.id IS NULL AND f.level = $2
		ORDER BY RANDOM()
		LIMIT $3`

	rows, err := r.db.Query(ctx, query, userID, level, limit)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения новых карточек: %w", err)
	}
	defer rows.Close()

	var flashcards []*models.Flashcard
	for rows.Next() {
		flashcard := &models.Flashcard{}
		err := rows.Scan(
			&flashcard.ID, &flashcard.Word, &flashcard.Translation,
			&flashcard.Example, &flashcard.Level, &flashcard.Category, &flashcard.CreatedAt,
		)
		if err != nil {
			r.logger.Error("ошибка сканирования новой карточки", zap.Error(err))
			continue
		}
		flashcards = append(flashcards, flashcard)
	}

	r.logger.Info("результат получения новых карточек",
		zap.Int64("user_id", userID),
		zap.String("level", level),
		zap.Int("found_cards", len(flashcards)))

	return flashcards, nil
}

// GetNextCardToReview получает следующую карточку для повторения (с ближайшим временем)
func (r *flashcardRepository) GetNextCardToReview(ctx context.Context, userID int64) (*models.UserFlashcard, error) {
	query := `
		SELECT uf.id, uf.user_id, uf.flashcard_id, uf.difficulty, uf.review_count, 
		       uf.correct_count, uf.last_reviewed_at, uf.next_review_at, uf.is_learned, uf.created_at,
		       f.id, f.word, f.translation, f.example, f.level, f.category, f.created_at
		FROM user_flashcards uf
		JOIN flashcards f ON uf.flashcard_id = f.id
		WHERE uf.user_id = $1 AND uf.is_learned = FALSE
		ORDER BY uf.next_review_at ASC
		LIMIT 1`

	row := r.db.QueryRow(ctx, query, userID)

	var userFlashcard models.UserFlashcard
	var flashcard models.Flashcard

	err := row.Scan(
		&userFlashcard.ID, &userFlashcard.UserID, &userFlashcard.FlashcardID,
		&userFlashcard.Difficulty, &userFlashcard.ReviewCount, &userFlashcard.CorrectCount,
		&userFlashcard.LastReviewedAt, &userFlashcard.NextReviewAt, &userFlashcard.IsLearned, &userFlashcard.CreatedAt,
		&flashcard.ID, &flashcard.Word, &flashcard.Translation,
		&flashcard.Example, &flashcard.Level, &flashcard.Category, &flashcard.CreatedAt,
	)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil // Нет карточек
		}
		return nil, fmt.Errorf("ошибка получения следующей карточки для повторения: %w", err)
	}

	userFlashcard.Flashcard = &flashcard
	return &userFlashcard, nil
}
