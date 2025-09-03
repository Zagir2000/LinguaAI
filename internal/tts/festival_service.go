package tts

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
)

// VoiceParameters содержит параметры для оптимизации голоса
type VoiceParameters struct {
	DurationStretch string
	IntTargetMean   string
	IntTargetStd    string
	F0Mean          string
	F0Std           string
	DurationFactor  string
}

// FestivalService предоставляет функциональность Text-to-Speech через Festival
type FestivalService struct {
	logger *zap.Logger
}

// NewFestivalService создает новый Festival TTS сервис
func NewFestivalService(logger *zap.Logger) *FestivalService {
	return &FestivalService{
		logger: logger,
	}
}

// SynthesizeText преобразует текст в аудио через Festival
func (s *FestivalService) SynthesizeText(ctx context.Context, text string) ([]byte, error) {
	// Ограничиваем длину текста
	if len(text) > 1000 {
		text = text[:1000] + "..."
	}

	// Очищаем текст от специальных символов
	cleanText := s.cleanText(text)

	// Проверяем, что Festival установлен
	if err := s.checkFestival(); err != nil {
		return nil, fmt.Errorf("festival не установлен: %w", err)
	}

	s.logger.Info("🎵 генерируем аудио через Festival",
		zap.String("text", cleanText),
		zap.Int("text_length", len(cleanText)))

	// Создаем контекст с таймаутом
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Генерируем аудио через Festival
	audioData, err := s.generateAudio(ctx, cleanText)
	if err != nil {
		return nil, fmt.Errorf("ошибка генерации аудио: %w", err)
	}

	s.logger.Info("🎵 аудио успешно сгенерировано",
		zap.String("text", cleanText),
		zap.Int("audio_size", len(audioData)))

	return audioData, nil
}

// checkFestival проверяет, что Festival установлен
func (s *FestivalService) checkFestival() error {
	cmd := exec.Command("festival", "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("festival не найден: %w", err)
	}

	s.logger.Debug("Festival версия", zap.String("version", string(output)))
	return nil
}

// getBestVoice возвращает лучший доступный голос
func (s *FestivalService) getBestVoice() string {
	// Список голосов в порядке предпочтения (от лучшего к худшему)
	voices := []string{
		"voice_us1_mbrola", // Американский мужской голос (MBROLA)
		"voice_us2_mbrola", // Американский женский голос (MBROLA)
		"voice_us3_mbrola", // Американский мужской голос (MBROLA)
		"voice_rablpc16k",  // Британский голос
		"voice_kallpc16k",  // Стандартный голос
	}

	// Проверяем доступность голосов
	for _, voice := range voices {
		cmd := exec.Command("festival", "-eval", fmt.Sprintf("(voice_%s)", voice), "-eval", "(exit)")
		if err := cmd.Run(); err == nil {
			s.logger.Info("🎤 Используем голос", zap.String("voice", voice))
			return voice
		}
	}

	// Если ничего не найдено, используем стандартный
	s.logger.Warn("🎤 Используем стандартный голос")
	return "voice_kallpc16k"
}

// getVoiceParameters возвращает оптимизированные параметры для конкретного голоса
func (s *FestivalService) getVoiceParameters(voice string) VoiceParameters {
	switch voice {
	case "voice_us1_mbrola":
		// Американский мужской голос (MBROLA) - глубокий и четкий
		return VoiceParameters{
			DurationStretch: "0.95",
			IntTargetMean:   "0.0",
			IntTargetStd:    "1.0",
			F0Mean:          "120", // Низкая частота для мужского голоса
			F0Std:           "20",
			DurationFactor:  "1.0",
		}
	case "voice_us2_mbrola":
		// Американский женский голос (MBROLA) - высокий и четкий
		return VoiceParameters{
			DurationStretch: "0.9",
			IntTargetMean:   "0.0",
			IntTargetStd:    "1.2",
			F0Mean:          "180", // Высокая частота для женского голоса
			F0Std:           "30",
			DurationFactor:  "1.0",
		}
	case "voice_us3_mbrola":
		// Американский мужской голос (MBROLA) - средний тон
		return VoiceParameters{
			DurationStretch: "0.95",
			IntTargetMean:   "0.0",
			IntTargetStd:    "1.1",
			F0Mean:          "140", // Средняя частота
			F0Std:           "25",
			DurationFactor:  "1.0",
		}
	case "voice_rablpc16k":
		// Британский голос - четкий и формальный
		return VoiceParameters{
			DurationStretch: "1.0",
			IntTargetMean:   "0.0",
			IntTargetStd:    "0.9",
			F0Mean:          "160",
			F0Std:           "20",
			DurationFactor:  "1.0",
		}
	default:
		// Стандартный голос
		return VoiceParameters{
			DurationStretch: "1.0",
			IntTargetMean:   "0.0",
			IntTargetStd:    "1.0",
			F0Mean:          "150",
			F0Std:           "25",
			DurationFactor:  "1.0",
		}
	}
}

// generateAudio генерирует аудио через Festival
func (s *FestivalService) generateAudio(ctx context.Context, text string) ([]byte, error) {
	// Создаем временный файл для текста
	tempTextFile := fmt.Sprintf("/tmp/festival_text_%d.txt", time.Now().UnixNano())

	// Записываем текст в файл
	if err := s.writeTextFile(tempTextFile, text); err != nil {
		return nil, fmt.Errorf("ошибка записи текста: %w", err)
	}
	defer s.cleanupFile(tempTextFile)

	// Создаем временный файл для аудио
	tempAudioFile := fmt.Sprintf("/tmp/festival_audio_%d.wav", time.Now().UnixNano())
	defer s.cleanupFile(tempAudioFile)

	// Получаем лучший доступный голос
	bestVoice := s.getBestVoice()

	// Получаем оптимизированные параметры для выбранного голоса
	voiceParams := s.getVoiceParameters(bestVoice)

	// Команда text2wave для генерации аудио с улучшенными параметрами качества
	cmd := exec.CommandContext(ctx, "text2wave",
		"-eval", fmt.Sprintf("(%s)", bestVoice), // Используем лучший доступный голос
		"-eval", fmt.Sprintf("(Parameter.set 'Duration_Stretch %s)", voiceParams.DurationStretch),
		"-eval", fmt.Sprintf("(Parameter.set 'Int_Target_Mean %s)", voiceParams.IntTargetMean),
		"-eval", fmt.Sprintf("(Parameter.set 'Int_Target_Std %s)", voiceParams.IntTargetStd),
		"-eval", fmt.Sprintf("(Parameter.set 'F0_Mean %s)", voiceParams.F0Mean),
		"-eval", fmt.Sprintf("(Parameter.set 'F0_Std %s)", voiceParams.F0Std),
		"-eval", fmt.Sprintf("(Parameter.set 'Duration_Factor %s)", voiceParams.DurationFactor),
		tempTextFile, "-o", tempAudioFile)

	// Перенаправляем вывод
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Выполняем команду
	if err := cmd.Run(); err != nil {
		s.logger.Error("ошибка выполнения text2wave",
			zap.Error(err),
			zap.String("stderr", stderr.String()))
		return nil, fmt.Errorf("ошибка выполнения text2wave: %w", err)
	}

	// Читаем сгенерированное аудио
	audioData, err := s.readAudioFile(tempAudioFile)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения аудио: %w", err)
	}

	return audioData, nil
}

// writeTextFile записывает текст в файл
func (s *FestivalService) writeTextFile(filename, text string) error {
	// Создаем файл
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Записываем текст
	_, err = file.WriteString(text)
	return err
}

// readAudioFile читает аудио файл
func (s *FestivalService) readAudioFile(filename string) ([]byte, error) {
	// Проверяем, что файл существует
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("аудио файл не найден: %s", filename)
	}

	// Читаем файл
	return os.ReadFile(filename)
}

// cleanupFile удаляет временный файл
func (s *FestivalService) cleanupFile(filename string) {
	if err := os.Remove(filename); err != nil {
		s.logger.Warn("ошибка удаления временного файла",
			zap.String("filename", filename),
			zap.Error(err))
	}
}

// cleanText очищает текст от специальных символов
func (s *FestivalService) cleanText(text string) string {
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
