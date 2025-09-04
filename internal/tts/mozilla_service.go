package tts

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
)

// MozillaService предоставляет функциональность Text-to-Speech через Mozilla TTS
type MozillaService struct {
	logger  *zap.Logger
	ttsPath string // Путь к исполняемому файлу TTS
}

// NewMozillaService создает новый Mozilla TTS сервис
func NewMozillaService(logger *zap.Logger) *MozillaService {
	return &MozillaService{
		logger: logger,
	}
}

// SynthesizeText преобразует текст в аудио через Mozilla TTS
func (s *MozillaService) SynthesizeText(ctx context.Context, text string) ([]byte, error) {
	// Ограничиваем длину текста для стабильности
	if len(text) > 500 {
		text = text[:500] + "..."
	}

	// Ограничиваем количество слов для предотвращения ошибок
	words := strings.Fields(text)
	if len(words) > 50 {
		text = strings.Join(words[:50], " ") + "..."
	}

	// Очищаем текст от специальных символов
	cleanText := s.cleanText(text)

	// Проверяем, что Mozilla TTS установлен
	if err := s.checkMozillaTTS(); err != nil {
		return nil, fmt.Errorf("mozilla tts не установлен: %w", err)
	}

	s.logger.Info("🎵 генерируем аудио через Mozilla TTS",
		zap.String("text", cleanText),
		zap.Int("text_length", len(cleanText)))

	// Создаем контекст с таймаутом
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Генерируем аудио через Mozilla TTS
	audioData, err := s.generateAudio(ctx, cleanText)
	if err != nil {
		return nil, fmt.Errorf("ошибка генерации аудио: %w", err)
	}

	s.logger.Info("🎵 аудио успешно сгенерировано",
		zap.String("text", cleanText),
		zap.Int("audio_size", len(audioData)))

	return audioData, nil
}

// checkMozillaTTS проверяет, что Mozilla TTS установлен
func (s *MozillaService) checkMozillaTTS() error {
	// Пробуем разные пути к TTS
	ttsPaths := []string{
		"tts",                                // Глобальный путь
		"/opt/lingua-ai-tts/tts_env/bin/tts", // Volume mount
	}

	var lastErr error
	for _, ttsPath := range ttsPaths {
		// Проверяем, что файл существует и исполняемый
		if _, err := os.Stat(ttsPath); err == nil {
			s.logger.Debug("mozilla tts найден",
				zap.String("path", ttsPath))
			// Сохраняем рабочий путь
			s.ttsPath = ttsPath
			return nil
		} else {
			lastErr = err
		}
	}

	return fmt.Errorf("mozilla tts не найден ни в одном из путей: %w", lastErr)
}

// generateAudio генерирует аудио через Mozilla TTS
func (s *MozillaService) generateAudio(ctx context.Context, text string) ([]byte, error) {
	// Создаем временный файл для аудио
	tempAudioFile := fmt.Sprintf("/tmp/mozilla_audio_%d", time.Now().UnixNano())
	defer s.cleanupFile(tempAudioFile)

	// Команда Mozilla TTS для генерации аудио
	ttsPath := s.ttsPath
	if ttsPath == "" {
		ttsPath = "tts" // Fallback
	}

	// Запускаем через Python из volume mount, так как tts - это Python скрипт
	cmd := exec.CommandContext(ctx, "/opt/lingua-ai-tts/tts_env/bin/python3", ttsPath,
		"--text", text,
		"--model_name", "tts_models/en/ljspeech/tacotron2-DDC",
		"--out_path", tempAudioFile)

	// Выполняем команду Mozilla TTS
	if err := cmd.Run(); err != nil {
		s.logger.Error("ошибка выполнения mozilla tts", zap.Error(err))
		return nil, fmt.Errorf("ошибка выполнения mozilla tts: %w", err)
	}

	// TTS создает файл с расширением .wa, проверяем оба варианта
	actualAudioFile := tempAudioFile + ".wa"
	if _, err := os.Stat(actualAudioFile); os.IsNotExist(err) {
		// Пробуем с расширением .wav
		actualAudioFile = tempAudioFile + ".wav"
		if _, err := os.Stat(actualAudioFile); os.IsNotExist(err) {
			s.logger.Error("аудио файл не был создан", zap.String("filename", tempAudioFile))
			return nil, fmt.Errorf("аудио файл не был создан: %s", tempAudioFile)
		}
	}

	// Читаем сгенерированное аудио
	audioData, err := s.readAudioFile(actualAudioFile)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения аудио: %w", err)
	}

	return audioData, nil
}

// readAudioFile читает аудио файл
func (s *MozillaService) readAudioFile(filename string) ([]byte, error) {
	// Проверяем, что файл существует
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("аудио файл не найден: %s", filename)
	}

	// Читаем файл
	return os.ReadFile(filename)
}

// cleanupFile удаляет временный файл
func (s *MozillaService) cleanupFile(filename string) {
	// Удаляем файл с расширением .wa
	if err := os.Remove(filename + ".wa"); err != nil {
		s.logger.Debug("файл .wa не найден для удаления", zap.String("filename", filename+".wa"))
	}
	// Удаляем файл с расширением .wav
	if err := os.Remove(filename + ".wav"); err != nil {
		s.logger.Debug("файл .wav не найден для удаления", zap.String("filename", filename+".wav"))
	}
}

// cleanText очищает текст от специальных символов
func (s *MozillaService) cleanText(text string) string {
	// Убираем HTML теги
	text = strings.ReplaceAll(text, "<b>", "")
	text = strings.ReplaceAll(text, "</b>", "")
	text = strings.ReplaceAll(text, "<i>", "")
	text = strings.ReplaceAll(text, "</i>", "")
	text = strings.ReplaceAll(text, "<tg-spoiler>", "")
	text = strings.ReplaceAll(text, "</tg-spoiler>", "")

	// Убираем эмодзи
	text = strings.ReplaceAll(text, "🎵", "")
	text = strings.ReplaceAll(text, "🇷🇺", "")
	text = strings.ReplaceAll(text, "🇺🇸", "")

	// Убираем лишние пробелы
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "  ", " ")

	return text
}
