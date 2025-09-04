package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

// AllTalkService предоставляет функциональность Text-to-Speech через AllTalk TTS
type AllTalkService struct {
	logger    *zap.Logger
	baseURL   string
	httpClient *http.Client
}

// NewAllTalkService создает новый AllTalk TTS сервис
func NewAllTalkService(logger *zap.Logger, baseURL string) *AllTalkService {
	return &AllTalkService{
		logger: logger,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// SynthesizeText преобразует текст в аудио через AllTalk TTS
func (s *AllTalkService) SynthesizeText(ctx context.Context, text string) ([]byte, error) {
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

	s.logger.Info("🎵 генерируем аудио через AllTalk TTS",
		zap.String("text", cleanText),
		zap.Int("text_length", len(cleanText)))

	// Генерируем аудио через AllTalk TTS
	audioData, err := s.generateAudio(ctx, cleanText)
	if err != nil {
		return nil, fmt.Errorf("ошибка генерации аудио: %w", err)
	}

	s.logger.Info("🎵 аудио успешно сгенерировано",
		zap.String("text", cleanText),
		zap.Int("audio_size", len(audioData)))

	return audioData, nil
}

// generateAudio генерирует аудио через AllTalk TTS API
func (s *AllTalkService) generateAudio(ctx context.Context, text string) ([]byte, error) {
	// Подготавливаем данные для запроса
	data := url.Values{}
	data.Set("text_input", text)
	data.Set("text_filtering", "standard")
	data.Set("character_voice_gen", "female_01.wav")
	data.Set("narrator_enabled", "false")
	data.Set("text_not_inside", "character")
	data.Set("language", "en")
	data.Set("output_file_name", fmt.Sprintf("tts_%d", time.Now().UnixNano()))
	data.Set("output_file_timestamp", "true")
	data.Set("autoplay", "false")
	data.Set("autoplay_volume", "0.8")

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/api/tts-generate", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	s.logger.Info("🎵 отправляем запрос к AllTalk TTS",
		zap.String("url", req.URL.String()),
		zap.String("text", text))

	// Выполняем запрос
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("AllTalk TTS вернул ошибку %d: %s", resp.StatusCode, string(body))
	}

	// Читаем ответ
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	// Парсим JSON ответ
	var response struct {
		Status         string `json:"status"`
		OutputFilePath string `json:"output_file_path"`
		OutputFileURL  string `json:"output_file_url"`
	}

	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	if response.Status != "generate-success" {
		return nil, fmt.Errorf("AllTalk TTS вернул статус: %s", response.Status)
	}

	// Скачиваем аудио файл
	audioData, err := s.downloadAudioFile(ctx, response.OutputFileURL)
	if err != nil {
		return nil, fmt.Errorf("ошибка скачивания аудио: %w", err)
	}

	return audioData, nil
}

// downloadAudioFile скачивает аудио файл по URL
func (s *AllTalkService) downloadAudioFile(ctx context.Context, audioURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", audioURL, nil)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса для скачивания: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка скачивания аудио файла: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ошибка скачивания аудио: статус %d", resp.StatusCode)
	}

	// Читаем аудио данные
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		return nil, fmt.Errorf("ошибка чтения аудио данных: %w", err)
	}

	return buf.Bytes(), nil
}

// cleanText очищает текст от специальных символов
func (s *AllTalkService) cleanText(text string) string {
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
