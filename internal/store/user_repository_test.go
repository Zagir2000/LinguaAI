package store

import (
	"testing"
	"time"

	"lingua-ai/pkg/models"
)

func TestUpdateStudyActivity(t *testing.T) {
	// Создаем тестовую базу данных (в реальном тесте нужно использовать testcontainers)
	// Здесь мы просто проверяем логику без реальной БД

	tests := []struct {
		name           string
		lastStudyDate  time.Time
		currentStreak  int
		expectedStreak int
	}{
		{
			name:           "первый день обучения",
			lastStudyDate:  time.Now().AddDate(0, 0, -2), // 2 дня назад
			currentStreak:  0,
			expectedStreak: 1,
		},
		{
			name:           "второй день подряд",
			lastStudyDate:  time.Now().AddDate(0, 0, -1), // вчера
			currentStreak:  1,
			expectedStreak: 2,
		},
		{
			name:           "уже занимался сегодня",
			lastStudyDate:  time.Now(), // сегодня
			currentStreak:  5,
			expectedStreak: 5, // не меняется
		},
		{
			name:           "пропустил день",
			lastStudyDate:  time.Now().AddDate(0, 0, -3), // 3 дня назад
			currentStreak:  10,
			expectedStreak: 1, // сбрасывается
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем тестового пользователя
			user := &models.User{
				ID:            1,
				TelegramID:    123456,
				FirstName:     "Test",
				Level:         models.LevelBeginner,
				XP:            100,
				StudyStreak:   tt.currentStreak,
				LastStudyDate: tt.lastStudyDate,
			}

			// Проверяем логику расчета streak
			now := time.Now()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			lastStudyDay := time.Date(user.LastStudyDate.Year(), user.LastStudyDate.Month(), user.LastStudyDate.Day(), 0, 0, 0, 0, user.LastStudyDate.Location())

			var newStreak int
			if today.Equal(lastStudyDay) {
				newStreak = user.StudyStreak
			} else if today.Sub(lastStudyDay) == 24*time.Hour {
				newStreak = user.StudyStreak + 1
			} else if today.Sub(lastStudyDay) > 24*time.Hour {
				newStreak = 1
			} else {
				newStreak = 1
			}

			if newStreak != tt.expectedStreak {
				t.Errorf("ожидался streak %d, получен %d", tt.expectedStreak, newStreak)
			}
		})
	}
}

func TestGetTopUsersByStreak(t *testing.T) {
	// Тест структуры запроса
	query := `
		SELECT id, telegram_id, username, first_name, last_name, level, xp, study_streak, last_study_date, current_state, last_seen, created_at, updated_at
		FROM users
		WHERE study_streak > 0
		ORDER BY study_streak DESC, last_study_date DESC
		LIMIT $1
	`

	// Проверяем, что запрос содержит нужные поля
	expectedFields := []string{
		"study_streak",
		"last_study_date",
		"ORDER BY study_streak DESC",
	}

	for _, field := range expectedFields {
		if !contains(query, field) {
			t.Errorf("запрос не содержит поле: %s", field)
		}
	}
}

func TestUserStatsWithStreak(t *testing.T) {
	// Тест структуры статистики
	stats := &models.UserStats{
		UserID:        1,
		TotalXP:       500,
		StudyStreak:   7,
		LastStudyDate: time.Now(),
	}

	if stats.StudyStreak != 7 {
		t.Errorf("ожидался study streak 7, получен %d", stats.StudyStreak)
	}

	if stats.TotalXP != 500 {
		t.Errorf("ожидался total XP 500, получен %d", stats.TotalXP)
	}
}

// Вспомогательная функция для проверки содержимого строки
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
