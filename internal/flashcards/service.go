package flashcards

import (
	"context"
	"fmt"
	"math"
	"time"

	"lingua-ai/internal/store"
	"lingua-ai/pkg/models"

	"go.uber.org/zap"
)

// Service сервис для работы со словарными карточками
type Service struct {
	flashcardRepo  store.FlashcardRepository
	logger         *zap.Logger
	activeSessions map[int64]*models.FlashcardSession // Активные сессии пользователей
}

// NewService создает новый сервис карточек
func NewService(flashcardRepo store.FlashcardRepository, logger *zap.Logger) *Service {
	return &Service{
		flashcardRepo:  flashcardRepo,
		logger:         logger,
		activeSessions: make(map[int64]*models.FlashcardSession),
	}
}

// StartFlashcardSession начинает новую сессию изучения карточек
func (s *Service) StartFlashcardSession(ctx context.Context, userID int64, userLevel string) (*models.FlashcardSession, error) {
	s.logger.Info("начинаем сессию карточек",
		zap.Int64("user_id", userID),
		zap.String("user_level", userLevel))

	// Получаем карточки для повторения
	cardsToReview, err := s.flashcardRepo.GetCardsToReview(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения карточек для повторения: %w", err)
	}

	s.logger.Info("карточки для повторения",
		zap.Int64("user_id", userID),
		zap.Int("cards_to_review_count", len(cardsToReview)))

	// Если нет карточек для повторения, добавляем новые
	if len(cardsToReview) == 0 {
		// Если уровень пустой, используем beginner по умолчанию
		if userLevel == "" {
			userLevel = "beginner"
			s.logger.Info("уровень пользователя пустой, используем beginner по умолчанию",
				zap.Int64("user_id", userID))
		}

		newCards, err := s.flashcardRepo.GetNewCardsForUser(ctx, userID, userLevel, 10)
		if err != nil {
			return nil, fmt.Errorf("ошибка получения новых карточек: %w", err)
		}

		s.logger.Info("получены новые карточки",
			zap.Int64("user_id", userID),
			zap.String("user_level", userLevel),
			zap.Int("new_cards_count", len(newCards)))

		// Создаем UserFlashcard записи для новых карточек
		for _, card := range newCards {
			userFlashcard := &models.UserFlashcard{
				UserID:       userID,
				FlashcardID:  card.ID,
				Difficulty:   0, // Начальная сложность
				ReviewCount:  0,
				CorrectCount: 0,
				NextReviewAt: time.Now(), // Доступна сразу
				IsLearned:    false,
				Flashcard:    card,
			}

			err = s.flashcardRepo.CreateUserFlashcard(ctx, userFlashcard)
			if err != nil {
				s.logger.Error("ошибка создания пользовательской карточки", zap.Error(err))
				continue
			}

			cardsToReview = append(cardsToReview, userFlashcard)
		}
	}

	if len(cardsToReview) == 0 {
		s.logger.Info("нет доступных карточек для изучения")
		return nil, nil
	}

	// Создаем сессию
	session := &models.FlashcardSession{
		UserID:         userID,
		CardsToReview:  make([]models.UserFlashcard, len(cardsToReview)),
		SessionStarted: time.Now(),
		CardsCompleted: 0,
		CorrectAnswers: 0,
	}

	// Копируем карточки в сессию
	for i, card := range cardsToReview {
		session.CardsToReview[i] = *card
	}

	// Устанавливаем первую карточку
	if len(session.CardsToReview) > 0 {
		session.CurrentCard = &session.CardsToReview[0]
	}

	// Сохраняем активную сессию
	s.activeSessions[userID] = session

	s.logger.Info("начата сессия карточек",
		zap.Int64("user_id", userID),
		zap.Int("cards_count", len(cardsToReview)))

	return session, nil
}

// GetCurrentSession получает текущую активную сессию пользователя
func (s *Service) GetCurrentSession(userID int64) *models.FlashcardSession {
	return s.activeSessions[userID]
}

// AnswerCard обрабатывает ответ пользователя на карточку
func (s *Service) AnswerCard(ctx context.Context, userID int64, isCorrect bool, difficulty int) (*models.FlashcardAnswer, error) {
	session := s.activeSessions[userID]
	if session == nil {
		return nil, fmt.Errorf("активная сессия не найдена")
	}

	if session.CurrentCard == nil {
		return nil, fmt.Errorf("текущая карточка не найдена")
	}

	currentCard := session.CurrentCard

	// Обновляем статистику карточки
	currentCard.ReviewCount++
	if isCorrect {
		currentCard.CorrectCount++
		session.CorrectAnswers++
	}

	// Вычисляем новую сложность и интервал повторения
	answer := s.calculateSpacedRepetition(currentCard, isCorrect, difficulty)

	// Обновляем карточку
	now := time.Now()
	currentCard.LastReviewedAt = &now
	currentCard.NextReviewAt = now.Add(answer.NextReviewIn)
	currentCard.Difficulty = answer.Difficulty

	// Если карточка выучена (достаточно повторений и хорошая точность)
	if currentCard.ReviewCount >= 3 &&
		float64(currentCard.CorrectCount)/float64(currentCard.ReviewCount) >= 0.7 {
		currentCard.IsLearned = true
	}

	// Сохраняем изменения в БД
	err := s.flashcardRepo.UpdateUserFlashcard(ctx, currentCard)
	if err != nil {
		return nil, fmt.Errorf("ошибка обновления карточки: %w", err)
	}

	// Переходим к следующей карточке
	session.CardsCompleted++
	if session.CardsCompleted < len(session.CardsToReview) {
		session.CurrentCard = &session.CardsToReview[session.CardsCompleted]
	} else {
		// Сессия завершена - сохраняем прогресс и очищаем
		s.EndSession(userID)
	}

	s.logger.Info("ответ на карточку обработан",
		zap.Int64("user_id", userID),
		zap.String("word", currentCard.Flashcard.Word),
		zap.Bool("correct", isCorrect),
		zap.Int("difficulty", answer.Difficulty))

	return answer, nil
}

// calculateSpacedRepetition вычисляет интервал повторения по алгоритму SM-2
func (s *Service) calculateSpacedRepetition(card *models.UserFlashcard, isCorrect bool, userDifficulty int) *models.FlashcardAnswer {
	// Алгоритм основан на SuperMemo SM-2

	var newDifficulty int
	var interval time.Duration

	if isCorrect {
		// Увеличиваем сложность при правильном ответе
		newDifficulty = card.Difficulty + 1
		if newDifficulty > 5 {
			newDifficulty = 5
		}

		// Интервалы для правильных ответов (в днях)
		switch newDifficulty {
		case 0, 1:
			interval = 1 * 24 * time.Hour // 1 день
		case 2:
			interval = 3 * 24 * time.Hour // 3 дня
		case 3:
			interval = 7 * 24 * time.Hour // 1 неделя
		case 4:
			interval = 14 * 24 * time.Hour // 2 недели
		case 5:
			interval = 30 * 24 * time.Hour // 1 месяц
		}

		// Корректируем интервал на основе пользовательской оценки сложности
		if userDifficulty <= 2 { // Легко
			interval = time.Duration(float64(interval) * 1.5)
		} else if userDifficulty >= 4 { // Сложно
			interval = time.Duration(float64(interval) * 0.7)
		}

	} else {
		// При неправильном ответе сбрасываем прогресс
		newDifficulty = max(0, card.Difficulty-1)
		interval = 10 * time.Minute // Повторить через 10 минут
	}

	// Добавляем случайность ±20% для избежания кучности
	randomFactor := 0.8 + (0.4 * float64(time.Now().UnixNano()%100) / 100.0)
	interval = time.Duration(float64(interval) * randomFactor)

	return &models.FlashcardAnswer{
		IsCorrect:    isCorrect,
		Difficulty:   newDifficulty,
		NextReviewIn: interval,
	}
}

// GetUserStats получает статистику пользователя по карточкам
func (s *Service) GetUserStats(ctx context.Context, userID int64) (map[string]interface{}, error) {
	stats, err := s.flashcardRepo.GetUserFlashcardStats(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения статистики: %w", err)
	}

	// Добавляем дополнительную информацию
	learnedCount, err := s.flashcardRepo.GetLearnedWordsCount(ctx, userID)
	if err != nil {
		s.logger.Error("ошибка получения количества выученных слов", zap.Error(err))
	} else {
		stats["learned_words"] = learnedCount
	}

	// Проверяем активную сессию
	session := s.activeSessions[userID]
	if session != nil {
		stats["active_session"] = true
		stats["session_progress"] = fmt.Sprintf("%d/%d", session.CardsCompleted, len(session.CardsToReview))
		stats["session_accuracy"] = float64(session.CorrectAnswers) / math.Max(float64(session.CardsCompleted), 1) * 100
	} else {
		stats["active_session"] = false
	}

	return stats, nil
}

// EndSession завершает активную сессию пользователя
func (s *Service) EndSession(userID int64) {
	session := s.activeSessions[userID]
	if session != nil {
		// Сохраняем прогресс всех карточек в сессии
		for i := range session.CardsToReview {
			card := &session.CardsToReview[i]
			if card.ReviewCount > 0 {
				// Обновляем карточку в БД
				err := s.flashcardRepo.UpdateUserFlashcard(context.Background(), card)
				if err != nil {
					s.logger.Error("ошибка сохранения прогресса карточки при завершении сессии",
						zap.Int64("user_id", userID),
						zap.String("word", card.Flashcard.Word),
						zap.Error(err))
				}
			}
		}
	}

	delete(s.activeSessions, userID)
	s.logger.Info("сессия карточек завершена", zap.Int64("user_id", userID))
}

// GetSessionProgress получает прогресс текущей сессии
func (s *Service) GetSessionProgress(userID int64) map[string]interface{} {
	session := s.activeSessions[userID]
	if session == nil {
		return map[string]interface{}{
			"active": false,
		}
	}

	progress := map[string]interface{}{
		"active":      true,
		"total_cards": len(session.CardsToReview),
		"completed":   session.CardsCompleted,
		"correct":     session.CorrectAnswers,
		"accuracy":    float64(session.CorrectAnswers) / math.Max(float64(session.CardsCompleted), 1) * 100,
		"remaining":   len(session.CardsToReview) - session.CardsCompleted,
	}

	if session.CurrentCard != nil {
		progress["current_word"] = session.CurrentCard.Flashcard.Word
	}

	return progress
}

// GetRecommendedStudyTime рекомендует время для изучения
func (s *Service) GetRecommendedStudyTime(ctx context.Context, userID int64) (string, error) {
	cardsToReview, err := s.flashcardRepo.GetCardsToReview(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("ошибка получения карточек для повторения: %w", err)
	}

	count := len(cardsToReview)
	if count == 0 {
		// Проверим когда будет доступна следующая карточка
		nextCard, err := s.flashcardRepo.GetNextCardToReview(ctx, userID)
		if err != nil {
			return "Сегодня нет карточек для повторения! 🎉", nil
		}

		if nextCard != nil {
			timeUntilNext := time.Until(nextCard.NextReviewAt)
			if timeUntilNext > 0 {
				if timeUntilNext < time.Hour {
					return fmt.Sprintf("Следующая карточка будет доступна через %d мин", int(timeUntilNext.Minutes())), nil
				} else if timeUntilNext < 24*time.Hour {
					return fmt.Sprintf("Следующая карточка будет доступна через %d ч", int(timeUntilNext.Hours())), nil
				} else {
					return fmt.Sprintf("Следующая карточка будет доступна через %d дн", int(timeUntilNext.Hours()/24)), nil
				}
			}
		}

		return "Сегодня нет карточек для повторения! 🎉", nil
	}

	// Примерно 30 секунд на карточку
	estimatedMinutes := (count * 30) / 60
	if estimatedMinutes < 1 {
		estimatedMinutes = 1
	}

	return fmt.Sprintf("Рекомендуемое время изучения: %d мин (%d карточек)", estimatedMinutes, count), nil
}

// max возвращает максимум из двух чисел
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
