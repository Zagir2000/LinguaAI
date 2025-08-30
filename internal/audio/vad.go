package audio

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// VADProcessor обрабатывает аудио с использованием Voice Activity Detection
type VADProcessor struct {
	logger *zap.Logger
}

// NewVADProcessor создает новый VAD процессор
func NewVADProcessor(logger *zap.Logger) *VADProcessor {
	return &VADProcessor{
		logger: logger,
	}
}

// SilenceSegment представляет сегмент тишины
type SilenceSegment struct {
	Start    float64 `json:"start"`    // Время начала в секундах
	Duration float64 `json:"duration"` // Длительность в секундах
}

// SpeechSegment представляет сегмент речи
type SpeechSegment struct {
	Start    float64 `json:"start"`     // Время начала в секундах
	End      float64 `json:"end"`       // Время окончания в секундах
	Duration float64 `json:"duration"`  // Длительность в секундах
	FilePath string  `json:"file_path"` // Путь к файлу сегмента
}

// DetectSilenceSegments анализирует аудиофайл и находит сегменты тишины
func (vad *VADProcessor) DetectSilenceSegments(inputFile string) ([]SilenceSegment, error) {
	vad.logger.Info("анализируем аудио на предмет тишины", zap.String("file", inputFile))

	// Создаем временный файл для вывода
	tempDir := os.TempDir()
	analysisFile := filepath.Join(tempDir, fmt.Sprintf("silence_analysis_%d.txt", time.Now().UnixNano()))
	defer os.Remove(analysisFile)

	// Команда FFmpeg для анализа тишины
	// -30dB - порог тишины, 0.5 - минимальная длительность тишины
	cmd := exec.Command("ffmpeg",
		"-i", inputFile,
		"-af", "silencedetect=noise=-30dB:d=0.5",
		"-f", "null",
		"-")

	// Перенаправляем stderr в файл (FFmpeg выводит информацию в stderr)
	stderrFile, err := os.Create(analysisFile)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания файла анализа: %w", err)
	}
	defer stderrFile.Close()

	cmd.Stderr = stderrFile

	err = cmd.Run()
	if err != nil {
		vad.logger.Warn("FFmpeg завершился с ошибкой (это нормально для анализа)", zap.Error(err))
	}

	// Читаем результаты анализа
	return vad.parseSilenceAnalysis(analysisFile)
}

// parseSilenceAnalysis парсит результаты анализа тишины из файла
func (vad *VADProcessor) parseSilenceAnalysis(analysisFile string) ([]SilenceSegment, error) {
	file, err := os.Open(analysisFile)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия файла анализа: %w", err)
	}
	defer file.Close()

	var segments []SilenceSegment
	scanner := bufio.NewScanner(file)

	// Регулярные выражения для парсинга
	silenceStartRe := regexp.MustCompile(`silence_start: ([\d.]+)`)
	silenceEndRe := regexp.MustCompile(`silence_end: ([\d.]+) \| silence_duration: ([\d.]+)`)

	var currentStart *float64

	for scanner.Scan() {
		line := scanner.Text()

		// Ищем начало тишины
		if matches := silenceStartRe.FindStringSubmatch(line); matches != nil {
			start, err := strconv.ParseFloat(matches[1], 64)
			if err == nil {
				currentStart = &start
			}
		}

		// Ищем конец тишины
		if matches := silenceEndRe.FindStringSubmatch(line); matches != nil && currentStart != nil {
			duration, err := strconv.ParseFloat(matches[2], 64)
			if err == nil {
				segments = append(segments, SilenceSegment{
					Start:    *currentStart,
					Duration: duration,
				})
			}
			currentStart = nil
		}
	}

	vad.logger.Info("найдено сегментов тишины", zap.Int("count", len(segments)))
	return segments, scanner.Err()
}

// SplitAudioBySilence разделяет аудио на сегменты речи, используя паузы
func (vad *VADProcessor) SplitAudioBySilence(inputFile string, maxSegmentDuration float64) ([]SpeechSegment, error) {
	vad.logger.Info("разделяем аудио по паузам",
		zap.String("file", inputFile),
		zap.Float64("max_duration", maxSegmentDuration))

	// Получаем общую длительность аудио
	totalDuration, err := vad.getAudioDuration(inputFile)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения длительности аудио: %w", err)
	}

	// Анализируем тишину
	silenceSegments, err := vad.DetectSilenceSegments(inputFile)
	if err != nil {
		return nil, fmt.Errorf("ошибка анализа тишины: %w", err)
	}

	// Создаем сегменты речи
	speechSegments := vad.createSpeechSegments(silenceSegments, totalDuration, maxSegmentDuration)

	// Извлекаем аудио сегменты
	outputDir := filepath.Dir(inputFile)
	baseName := strings.TrimSuffix(filepath.Base(inputFile), filepath.Ext(inputFile))

	for i, segment := range speechSegments {
		outputFile := filepath.Join(outputDir, fmt.Sprintf("%s_segment_%03d.wav", baseName, i))

		err := vad.extractAudioSegment(inputFile, outputFile, segment.Start, segment.Duration)
		if err != nil {
			vad.logger.Error("ошибка извлечения сегмента",
				zap.Int("segment", i),
				zap.Error(err))
			continue
		}

		speechSegments[i].FilePath = outputFile
	}

	vad.logger.Info("создано сегментов речи", zap.Int("count", len(speechSegments)))
	return speechSegments, nil
}

// getAudioDuration получает длительность аудиофайла
func (vad *VADProcessor) getAudioDuration(inputFile string) (float64, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		inputFile)

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ошибка выполнения ffprobe: %w", err)
	}

	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("ошибка парсинга длительности: %w", err)
	}

	return duration, nil
}

// createSpeechSegments создает сегменты речи на основе пауз
func (vad *VADProcessor) createSpeechSegments(silenceSegments []SilenceSegment, totalDuration, maxSegmentDuration float64) []SpeechSegment {
	var speechSegments []SpeechSegment

	currentStart := 0.0

	for _, silence := range silenceSegments {
		// Если есть речь перед этой паузой
		if silence.Start > currentStart {
			speechDuration := silence.Start - currentStart

			// Если сегмент слишком длинный, разбиваем его
			if speechDuration > maxSegmentDuration {
				segments := vad.splitLongSegment(currentStart, speechDuration, maxSegmentDuration)
				speechSegments = append(speechSegments, segments...)
			} else if speechDuration > 1.0 { // Игнорируем очень короткие сегменты
				speechSegments = append(speechSegments, SpeechSegment{
					Start:    currentStart,
					End:      silence.Start,
					Duration: speechDuration,
				})
			}
		}

		// Следующий сегмент начинается после паузы
		currentStart = silence.Start + silence.Duration
	}

	// Обрабатываем оставшуюся часть аудио
	if currentStart < totalDuration {
		remainingDuration := totalDuration - currentStart
		if remainingDuration > maxSegmentDuration {
			segments := vad.splitLongSegment(currentStart, remainingDuration, maxSegmentDuration)
			speechSegments = append(speechSegments, segments...)
		} else if remainingDuration > 1.0 {
			speechSegments = append(speechSegments, SpeechSegment{
				Start:    currentStart,
				End:      totalDuration,
				Duration: remainingDuration,
			})
		}
	}

	return speechSegments
}

// splitLongSegment разбивает длинный сегмент на более короткие
func (vad *VADProcessor) splitLongSegment(start, duration, maxDuration float64) []SpeechSegment {
	var segments []SpeechSegment

	currentStart := start
	remaining := duration

	for remaining > 0 {
		segmentDuration := maxDuration
		if remaining < maxDuration {
			segmentDuration = remaining
		}

		segments = append(segments, SpeechSegment{
			Start:    currentStart,
			End:      currentStart + segmentDuration,
			Duration: segmentDuration,
		})

		currentStart += segmentDuration
		remaining -= segmentDuration
	}

	return segments
}

// extractAudioSegment извлекает сегмент аудио с помощью FFmpeg
func (vad *VADProcessor) extractAudioSegment(inputFile, outputFile string, start, duration float64) error {
	cmd := exec.Command("ffmpeg",
		"-i", inputFile,
		"-ss", fmt.Sprintf("%.3f", start),
		"-t", fmt.Sprintf("%.3f", duration),
		"-ar", "16000", // Whisper требует 16kHz
		"-ac", "1", // Моно
		"-y", // Перезаписать файл
		outputFile)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("ошибка извлечения сегмента: %w", err)
	}

	return nil
}

// CleanupSegments удаляет временные файлы сегментов
func (vad *VADProcessor) CleanupSegments(segments []SpeechSegment) {
	for _, segment := range segments {
		if segment.FilePath != "" {
			os.Remove(segment.FilePath)
		}
	}
}
