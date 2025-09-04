package tts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// PiperService предоставляет функциональность Text-to-Speech через Piper TTS API
type PiperService struct {
	logger  *zap.Logger
	baseURL string
	client  *http.Client
}

// NewPiperService создает новый Piper TTS сервис
func NewPiperService(logger *zap.Logger, baseURL string) *PiperService {
	return &PiperService{
		logger:  logger,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second, // Таймаут для генерации аудио
		},
	}
}

// SynthesizeText преобразует текст в аудио через Piper TTS
func (s *PiperService) SynthesizeText(ctx context.Context, text string) ([]byte, error) {
	s.logger.Info("🎵 генерируем аудио через Piper TTS",
		zap.String("text", text),
		zap.Int("text_length", len(text)))

	audioData, err := s.generateAudio(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("ошибка генерации аудио: %w", err)
	}

	s.logger.Info("🎵 аудио успешно сгенерировано",
		zap.String("text", text),
		zap.Int("audio_size", len(audioData)))

	return audioData, nil
}

// generateAudio отправляет запрос к Piper TTS API и получает аудио
func (s *PiperService) generateAudio(ctx context.Context, text string) ([]byte, error) {
	url := fmt.Sprintf("%s/synthesize-raw", s.baseURL)

	// Создаем multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Добавляем поля формы
	_ = writer.WriteField("text", text)
	// language будет определен автоматически по тексту

	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	s.logger.Info("🎵 отправляем запрос к Piper TTS",
		zap.String("url", url),
		zap.String("text", text))

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("неожиданный статус от Piper TTS: %d, тело: %s", resp.StatusCode, respBody)
	}

	// Читаем аудио данные
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения аудио данных: %w", err)
	}

	return audioData, nil
}
