package whisper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"lingua-ai/internal/audio"
)

// Client представляет клиент для работы с Whisper API
type Client struct {
	apiURL       string
	httpClient   *http.Client
	logger       *zap.Logger
	vadProcessor *audio.VADProcessor
}

// NewClient создает новый клиент Whisper
func NewClient(apiURL string, logger *zap.Logger) *Client {
	return &Client{
		apiURL: apiURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // Увеличиваем таймаут для обработки аудио
		},
		logger:       logger,
		vadProcessor: audio.NewVADProcessor(logger),
	}
}

// TranscribeRequest представляет запрос на транскрибацию
type TranscribeRequest struct {
	AudioFile string `json:"audio_file"`
	Language  string `json:"language,omitempty"`  // auto, en, ru
	Format    string `json:"format,omitempty"`    // json, txt, srt, vtt
	Task      string `json:"task,omitempty"`      // transcribe, translate
	BeamSize  int    `json:"beam_size,omitempty"` // Размер луча для поиска
	BestOf    int    `json:"best_of,omitempty"`   // Количество лучших кандидатов
}

// TranscribeResponse представляет ответ от Whisper API
type TranscribeResponse struct {
	Text     string  `json:"text"`
	Language string  `json:"language"`
	Duration float64 `json:"duration"`
	Segments []struct {
		Start  float64 `json:"start"`
		End    float64 `json:"end"`
		Text   string  `json:"text"`
		Tokens []int   `json:"tokens"`
	} `json:"segments"`
}

// TranscribeFile транскрибирует аудио файл
func (c *Client) TranscribeFile(ctx context.Context, filePath string) (*TranscribeResponse, error) {
	// Проверяем существование файла
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("аудио файл не найден: %s", filePath)
	}

	// Открываем файл
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия файла: %w", err)
	}
	defer file.Close()

	// Создаем multipart запрос
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Добавляем файл
	part, err := writer.CreateFormFile("audio_file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("ошибка создания формы: %w", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, fmt.Errorf("ошибка копирования файла: %w", err)
	}

	writer.Close()

	// Создаем HTTP запрос с поддерживаемыми параметрами
	apiURL := c.apiURL + "/asr"
	params := []string{
		"output=json",
		"task=transcribe", // Задача: транскрибация
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL+"?"+strings.Join(params, "&"), &requestBody)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	c.logger.Info("отправка запроса на транскрибацию",
		zap.String("file", filePath),
		zap.String("api_url", c.apiURL),
		zap.String("params", strings.Join(params, "&")))

	// Отправляем запрос
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка отправки запроса: %w", err)
	}
	defer resp.Body.Close()

	// Читаем ответ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		// Пытаемся парсить как JSON для детальных ошибок
		var errorResponse map[string]interface{}
		if json.Unmarshal(body, &errorResponse) == nil {
			// Если это JSON, возвращаем детальную ошибку
			errorJSON, _ := json.Marshal(errorResponse)
			return nil, fmt.Errorf("ошибка API (статус %d): %s", resp.StatusCode, string(errorJSON))
		}
		// Если не JSON, возвращаем как есть
		return nil, fmt.Errorf("ошибка API (статус %d): %s", resp.StatusCode, string(body))
	}

	// Проверяем Content-Type, но разрешаем text/plain если это JSON
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") && !strings.Contains(contentType, "text/plain") {
		return nil, fmt.Errorf("неожиданный Content-Type: %s, тело: %s", contentType, string(body))
	}

	// Парсим ответ
	var response TranscribeResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w, тело: %s", err, string(body))
	}

	c.logger.Info("транскрибация завершена",
		zap.String("file", filePath),
		zap.String("text", response.Text),
		zap.Float64("duration", response.Duration))

	return &response, nil
}

// TranscribeBytes транскрибирует аудио данные из байтов
func (c *Client) TranscribeBytes(ctx context.Context, audioData []byte, filename string) (*TranscribeResponse, error) {
	// Создаем multipart запрос
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Добавляем файл
	part, err := writer.CreateFormFile("audio_file", filename)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания формы: %w", err)
	}

	_, err = part.Write(audioData)
	if err != nil {
		return nil, fmt.Errorf("ошибка записи данных: %w", err)
	}

	writer.Close()

	// Создаем HTTP запрос с поддерживаемыми параметрами
	apiURL := c.apiURL + "/asr"
	params := []string{
		"output=json",
		"task=transcribe", // Задача: транскрибация
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL+"?"+strings.Join(params, "&"), &requestBody)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	c.logger.Info("отправка запроса на транскрибацию байтов",
		zap.String("filename", filename),
		zap.Int("size", len(audioData)),
		zap.String("params", strings.Join(params, "&")))

	// Отправляем запрос
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ошибка отправки запроса: %w", err)
	}
	defer resp.Body.Close()

	// Читаем ответ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		// Пытаемся парсить как JSON для детальных ошибок
		var errorResponse map[string]interface{}
		if json.Unmarshal(body, &errorResponse) == nil {
			// Если это JSON, возвращаем детальную ошибку
			errorJSON, _ := json.Marshal(errorResponse)
			return nil, fmt.Errorf("ошибка API (статус %d): %s", resp.StatusCode, string(errorJSON))
		}
		// Если не JSON, возвращаем как есть
		return nil, fmt.Errorf("ошибка API (статус %d): %s", resp.StatusCode, string(body))
	}

	// Проверяем Content-Type, но разрешаем text/plain если это JSON
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") && !strings.Contains(contentType, "text/plain") {
		return nil, fmt.Errorf("неожиданный Content-Type: %s, тело: %s", contentType, string(body))
	}

	// Парсим ответ
	var response TranscribeResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w, тело: %s", err, string(body))
	}

	c.logger.Info("транскрибация байтов завершена",
		zap.String("filename", filename),
		zap.String("text", response.Text),
		zap.Float64("duration", response.Duration))

	return &response, nil
}

// HealthCheck проверяет доступность Whisper API
func (c *Client) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.apiURL+"/", nil)
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка отправки запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("нездоровый статус API: %d", resp.StatusCode)
	}

	return nil
}
// TranscribeWithVAD транскрибирует аудио с использованием Voice Activity Detection
func (c *Client) TranscribeWithVAD(ctx context.Context, audioFilePath string, req TranscribeRequest) (*TranscribeResponse, error) {
	c.logger.Info("начинаем транскрибацию с VAD", zap.String("file", audioFilePath))

	// Разделяем аудио на сегменты с помощью VAD
	maxSegmentDuration := 30.0 // 30 секунд - оптимально для Whisper
	segments, err := c.vadProcessor.SplitAudioBySilence(audioFilePath, maxSegmentDuration)
	if err != nil {
		return nil, fmt.Errorf("ошибка разделения аудио на сегменты: %w", err)
	}

	if len(segments) == 0 {
		c.logger.Warn("не найдено сегментов речи в аудио")
		// Fallback к обычной транскрибации
		return c.TranscribeFile(ctx, audioFilePath)
	}

	c.logger.Info("найдено сегментов речи", zap.Int("count", len(segments)))

	// Очищаем временные файлы в конце
	defer c.vadProcessor.CleanupSegments(segments)

	var allTranscriptions []string
	var detectedLanguage string

	// Транскрибируем каждый сегмент
	for i, segment := range segments {
		if segment.FilePath == "" {
			c.logger.Warn("пропускаем сегмент без файла", zap.Int("segment", i))
			continue
		}

		c.logger.Debug("транскрибируем сегмент",
			zap.Int("segment", i),
			zap.Float64("start", segment.Start),
			zap.Float64("duration", segment.Duration))

		segmentReq := req
		segmentReq.AudioFile = segment.FilePath

		response, err := c.TranscribeFile(ctx, segment.FilePath)
		if err != nil {
			c.logger.Error("ошибка транскрибации сегмента",
				zap.Int("segment", i),
				zap.Error(err))
			continue
		}

		if response.Text != "" {
			allTranscriptions = append(allTranscriptions, strings.TrimSpace(response.Text))
		}

		// Сохраняем язык из первого успешно обработанного сегмента
		if detectedLanguage == "" && response.Language != "" {
			detectedLanguage = response.Language
		}
	}

	if len(allTranscriptions) == 0 {
		return nil, fmt.Errorf("не удалось транскрибировать ни одного сегмента")
	}

	// Объединяем результаты
	finalText := strings.Join(allTranscriptions, " ")

	c.logger.Info("транскрибация с VAD завершена",
		zap.Int("segments_processed", len(allTranscriptions)),
		zap.Int("total_segments", len(segments)),
		zap.String("detected_language", detectedLanguage),
		zap.Int("text_length", len(finalText)))

	return &TranscribeResponse{
		Text:     finalText,
		Language: detectedLanguage,
	}, nil
}

// TranscribeOptions содержит настройки для транскрибации
type TranscribeOptions struct {
	UseVAD             bool    `json:"use_vad"`              // Использовать VAD для разделения
	MaxSegmentDuration float64 `json:"max_segment_duration"` // Максимальная длительность сегмента
	Language           string  `json:"language"`             // Язык для транскрибации
	Task               string  `json:"task"`                 // transcribe или translate
}

// TranscribeAdvanced выполняет транскрибацию с расширенными настройками
func (c *Client) TranscribeAdvanced(ctx context.Context, audioFilePath string, options TranscribeOptions) (*TranscribeResponse, error) {
	req := TranscribeRequest{
		Language: options.Language,
		Task:     options.Task,
		Format:   "json",
	}

	if options.UseVAD {
		return c.TranscribeWithVAD(ctx, audioFilePath, req)
	}

	return c.TranscribeFile(ctx, audioFilePath)
}

