package models

import (
	"time"
)

// User представляет пользователя в системе
type User struct {
	ID                int64      `json:"id" db:"id"`
	TelegramID        int64      `json:"telegram_id" db:"telegram_id"`
	Username          string     `json:"username" db:"username"`
	FirstName         string     `json:"first_name" db:"first_name"`
	LastName          string     `json:"last_name" db:"last_name"`
	Level             string     `json:"level" db:"level"` // beginner, intermediate, advanced
	XP                int        `json:"xp" db:"xp"`
	StudyStreak       int        `json:"study_streak" db:"study_streak"`       // дни подряд обучения
	LastStudyDate     time.Time  `json:"last_study_date" db:"last_study_date"` // последняя дата обучения
	CurrentState      string     `json:"current_state" db:"current_state"`     // idle, in_level_test
	LastSeen          time.Time  `json:"last_seen" db:"last_seen"`
	IsPremium         bool       `json:"is_premium" db:"is_premium"`                   // Статус премиум-подписки
	PremiumExpiresAt  *time.Time `json:"premium_expires_at" db:"premium_expires_at"`   // Дата истечения премиума
	MessagesCount     int        `json:"messages_count" db:"messages_count"`           // Количество отправленных сообщений
	MaxMessages       int        `json:"max_messages" db:"max_messages"`               // Максимум сообщений для бесплатных
	MessagesResetDate time.Time  `json:"messages_reset_date" db:"messages_reset_date"` // Дата последнего сброса счетчика
	LastTestDate      *time.Time `json:"last_test_date" db:"last_test_date"`           // Дата последнего теста уровня
	ReferralCode      *string    `json:"referral_code" db:"referral_code"`             // Уникальный реферальный код
	ReferralCount     int        `json:"referral_count" db:"referral_count"`           // Количество приглашенных пользователей

	ReferredBy *int64    `json:"referred_by" db:"referred_by"` // ID пользователя, который пригласил
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// UserMessage представляет сообщение в диалоге
type UserMessage struct {
	ID        int64     `json:"id" db:"id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	Role      string    `json:"role" db:"role"` // "user" или "assistant"
	Content   string    `json:"content" db:"content"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// UserStats представляет статистику пользователя
type UserStats struct {
	UserID        int64     `json:"user_id" db:"user_id"`
	TotalXP       int       `json:"total_xp" db:"total_xp"`
	StudyStreak   int       `json:"study_streak" db:"study_streak"` // дни подряд
	LastStudyDate time.Time `json:"last_study_date" db:"last_study_date"`
}

// CreateUserRequest представляет запрос на создание пользователя
type CreateUserRequest struct {
	TelegramID int64  `json:"telegram_id" validate:"required"`
	Username   string `json:"username"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
}

// UpdateUserRequest представляет запрос на обновление пользователя
type UpdateUserRequest struct {
	Level             *string    `json:"level,omitempty"`
	XP                *int       `json:"xp,omitempty"`
	LastSeen          *time.Time `json:"last_seen,omitempty"`
	CurrentState      *string    `json:"current_state,omitempty"`
	IsPremium         *bool      `json:"is_premium,omitempty"`
	PremiumExpiresAt  *time.Time `json:"premium_expires_at,omitempty"`
	MessagesCount     *int       `json:"messages_count,omitempty"`
	MaxMessages       *int       `json:"max_messages,omitempty"`
	MessagesResetDate *time.Time `json:"messages_reset_date,omitempty"`
	LastTestDate      *time.Time `json:"last_test_date,omitempty"`
	ReferralCode      *string    `json:"referral_code,omitempty"`
	ReferralCount     *int       `json:"referral_count,omitempty"`

	ReferredBy *int64 `json:"referred_by,omitempty"`
}

// CreateMessageRequest представляет запрос на создание сообщения
type CreateMessageRequest struct {
	UserID  int64  `json:"user_id" validate:"required"`
	Role    string `json:"role" validate:"required,oneof=user assistant"`
	Content string `json:"content" validate:"required"`
}

// ChatHistory представляет историю диалога
type ChatHistory struct {
	Messages []UserMessage `json:"messages"`
	User     *User         `json:"user,omitempty"`
}

// LevelTest представляет тест для определения уровня английского
type LevelTest struct {
	UserID          int64               `json:"user_id"`
	CurrentQuestion int                 `json:"current_question"`
	Questions       []LevelTestQuestion `json:"questions"`
	Answers         []LevelTestAnswer   `json:"answers"`
	Score           int                 `json:"score"`
	MaxScore        int                 `json:"max_score"`
	StartedAt       time.Time           `json:"started_at"`
	CompletedAt     *time.Time          `json:"completed_at,omitempty"`
}

// LevelTestQuestion представляет вопрос теста уровня
type LevelTestQuestion struct {
	ID            int      `json:"id"`
	Question      string   `json:"question"`
	Options       []string `json:"options"`
	CorrectAnswer int      `json:"correct_answer"`
	Level         string   `json:"level"` // beginner, intermediate, advanced
	Points        int      `json:"points"`
}

// LevelTestAnswer представляет ответ пользователя на вопрос теста
type LevelTestAnswer struct {
	QuestionID int  `json:"question_id"`
	Answer     int  `json:"answer"`
	IsCorrect  bool `json:"is_correct"`
	Points     int  `json:"points"`
}

// Payment представляет платеж за премиум-подписку
type Payment struct {
	ID                  int64          `json:"id" db:"id"`
	UserID              int64          `json:"user_id" db:"user_id"`
	Amount              float64        `json:"amount" db:"amount"`
	Currency            string         `json:"currency" db:"currency"`
	PaymentID           string         `json:"payment_id" db:"payment_id"` // ID от ЮKassa
	Status              string         `json:"status" db:"status"`         // pending, completed, failed, cancelled
	PremiumDurationDays int            `json:"premium_duration_days" db:"premium_duration_days"`
	CreatedAt           time.Time      `json:"created_at" db:"created_at"`
	CompletedAt         *time.Time     `json:"completed_at" db:"completed_at"`
	Metadata            map[string]any `json:"metadata" db:"metadata"` // Дополнительные данные
}

// PremiumPlan представляет план премиум-подписки
type PremiumPlan struct {
	ID           int      `json:"id"`
	Name         string   `json:"name"`
	DurationDays int      `json:"duration_days"`
	Price        float64  `json:"price"`
	Currency     string   `json:"currency"`
	Description  string   `json:"description"`
	Features     []string `json:"features"`
}

// CreatePaymentRequest представляет запрос на создание платежа
type CreatePaymentRequest struct {
	UserID              int64   `json:"user_id" validate:"required"`
	Amount              float64 `json:"amount" validate:"required,gt=0"`
	Currency            string  `json:"currency" validate:"required"`
	PremiumDurationDays int     `json:"premium_duration_days" validate:"required,gt=0"`
}

// UpdatePaymentRequest представляет запрос на обновление платежа
type UpdatePaymentRequest struct {
	Status      *string         `json:"status,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	Metadata    *map[string]any `json:"metadata,omitempty"`
}

// Constants для уровней пользователей
const (
	LevelBeginner     = "beginner"
	LevelIntermediate = "intermediate"
	LevelAdvanced     = "advanced"
)

// Constants для порогов XP уровней
const (
	XPThresholdBeginner     = 0     // 0 - 9,999 XP
	XPThresholdIntermediate = 10000 // 10,000 - 19,999 XP
	XPThresholdAdvanced     = 20000 // 20,000+ XP
)

// Constants для ролей сообщений
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

// Constants для состояний пользователя
const (
	StateIdle         = "idle"
	StateInLevelTest  = "in_level_test"
	StateInFlashcards = "in_flashcards"
)

// IsValidLevel проверяет корректность уровня пользователя
func IsValidLevel(level string) bool {
	switch level {
	case LevelBeginner, LevelIntermediate, LevelAdvanced:
		return true
	default:
		return false
	}
}

// IsValidRole проверяет корректность роли сообщения
func IsValidRole(role string) bool {
	switch role {
	case RoleUser, RoleAssistant, RoleSystem:
		return true
	default:
		return false
	}
}

// Flashcard представляет словарную карточку
type Flashcard struct {
	ID          int64     `json:"id" db:"id"`
	Word        string    `json:"word" db:"word"`
	Translation string    `json:"translation" db:"translation"`
	Example     string    `json:"example" db:"example"`
	Level       string    `json:"level" db:"level"`       // beginner, intermediate, advanced
	Category    string    `json:"category" db:"category"` // general, business, travel, etc.
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// UserFlashcard представляет прогресс пользователя по конкретной карточке
type UserFlashcard struct {
	ID             int64      `json:"id" db:"id"`
	UserID         int64      `json:"user_id" db:"user_id"`
	FlashcardID    int64      `json:"flashcard_id" db:"flashcard_id"`
	Difficulty     int        `json:"difficulty" db:"difficulty"`       // 0-5 (сложность для пользователя)
	ReviewCount    int        `json:"review_count" db:"review_count"`   // Количество повторений
	CorrectCount   int        `json:"correct_count" db:"correct_count"` // Количество правильных ответов
	LastReviewedAt *time.Time `json:"last_reviewed_at" db:"last_reviewed_at"`
	NextReviewAt   time.Time  `json:"next_review_at" db:"next_review_at"` // Когда нужно повторить
	IsLearned      bool       `json:"is_learned" db:"is_learned"`         // Выучено ли слово
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`

	// Связанная карточка (для JOIN запросов)
	Flashcard *Flashcard `json:"flashcard,omitempty" db:"-"`
}

// FlashcardSession представляет сессию изучения карточек
type FlashcardSession struct {
	UserID         int64           `json:"user_id"`
	CurrentCard    *UserFlashcard  `json:"current_card"`
	CardsToReview  []UserFlashcard `json:"cards_to_review"`
	SessionStarted time.Time       `json:"session_started"`
	CardsCompleted int             `json:"cards_completed"`
	CorrectAnswers int             `json:"correct_answers"`
}

// FlashcardAnswer представляет ответ пользователя на карточку
type FlashcardAnswer struct {
	IsCorrect    bool          `json:"is_correct"`
	Difficulty   int           `json:"difficulty"`     // Новая сложность (1-5)
	NextReviewIn time.Duration `json:"next_review_in"` // Через сколько повторить
}

// IsValidState проверяет корректность состояния пользователя
func IsValidState(state string) bool {
	switch state {
	case StateIdle, StateInLevelTest, StateInFlashcards:
		return true
	default:
		return false
	}
}

// GetLevelByXP определяет уровень пользователя на основе его XP
func GetLevelByXP(xp int) string {
	if xp >= XPThresholdAdvanced {
		return LevelAdvanced
	} else if xp >= XPThresholdIntermediate {
		return LevelIntermediate
	} else {
		return LevelBeginner
	}
}

// GetXPForNextLevel возвращает количество XP до следующего уровня
func GetXPForNextLevel(currentXP int) (int, string) {
	currentLevel := GetLevelByXP(currentXP)

	switch currentLevel {
	case LevelBeginner:
		return XPThresholdIntermediate - currentXP, LevelIntermediate
	case LevelIntermediate:
		return XPThresholdAdvanced - currentXP, LevelAdvanced
	case LevelAdvanced:
		return 0, LevelAdvanced // Уже максимальный уровень
	default:
		return XPThresholdIntermediate - currentXP, LevelIntermediate
	}
}

// GetLevelProgress возвращает прогресс в текущем уровне (в процентах)
func GetLevelProgress(xp int) float64 {
	currentLevel := GetLevelByXP(xp)

	switch currentLevel {
	case LevelBeginner:
		// Прогресс от 0 до 10000
		return float64(xp) / float64(XPThresholdIntermediate) * 100
	case LevelIntermediate:
		// Прогресс от 10000 до 20000
		progressXP := xp - XPThresholdIntermediate
		levelRangeXP := XPThresholdAdvanced - XPThresholdIntermediate
		return float64(progressXP) / float64(levelRangeXP) * 100
	case LevelAdvanced:
		// Для продвинутого уровня показываем полный прогресс
		return 100.0
	default:
		return 0.0
	}
}
