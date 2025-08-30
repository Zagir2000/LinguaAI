package whisper

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestNewClient(t *testing.T) {
	logger := zap.NewNop()
	client := NewClient("http://localhost:8080", logger)

	if client == nil {
		t.Fatal("клиент не должен быть nil")
	}

	if client.apiURL != "http://localhost:8080" {
		t.Errorf("ожидался apiURL 'http://localhost:8080', получен '%s'", client.apiURL)
	}

	if client.httpClient == nil {
		t.Error("httpClient не должен быть nil")
	}

	if client.logger == nil {
		t.Error("logger не должен быть nil")
	}
}

func TestHealthCheck(t *testing.T) {
	logger := zap.NewNop()
	client := NewClient("http://localhost:8080", logger)

	// Тест с несуществующим сервером должен вернуть ошибку
	ctx := context.Background()
	err := client.HealthCheck(ctx)

	if err == nil {
		t.Error("ожидалась ошибка при проверке несуществующего сервера")
	}
}

func TestTranscribeFile_FileNotExists(t *testing.T) {
	logger := zap.NewNop()
	client := NewClient("http://localhost:8080", logger)

	ctx := context.Background()
	_, err := client.TranscribeFile(ctx, "nonexistent.wav")

	if err == nil {
		t.Error("ожидалась ошибка для несуществующего файла")
	}

	expectedError := "аудио файл не найден"
	if err.Error()[:len(expectedError)] != expectedError {
		t.Errorf("ожидалась ошибка содержащая '%s', получена '%s'", expectedError, err.Error())
	}
}

func TestTranscribeBytes(t *testing.T) {
	logger := zap.NewNop()
	client := NewClient("http://localhost:8080", logger)

	ctx := context.Background()
	audioData := []byte("fake audio data")
	filename := "test.wav"

	// Тест с несуществующим сервером должен вернуть ошибку
	_, err := client.TranscribeBytes(ctx, audioData, filename)

	if err == nil {
		t.Error("ожидалась ошибка при отправке на несуществующий сервер")
	}
}

func TestTranscribeResponse_JSON(t *testing.T) {
	// Тест структуры ответа
	response := TranscribeResponse{
		Text:     "Hello world",
		Language: "en",
		Duration: 2.5,
		Segments: []struct {
			Start  float64 `json:"start"`
			End    float64 `json:"end"`
			Text   string  `json:"text"`
			Tokens []int   `json:"tokens"`
		}{
			{
				Start:  0.0,
				End:    2.5,
				Text:   "Hello world",
				Tokens: []int{1, 2, 3},
			},
		},
	}

	if response.Text != "Hello world" {
		t.Errorf("ожидался текст 'Hello world', получен '%s'", response.Text)
	}

	if response.Language != "en" {
		t.Errorf("ожидался язык 'en', получен '%s'", response.Language)
	}

	if response.Duration != 2.5 {
		t.Errorf("ожидалась длительность 2.5, получена %f", response.Duration)
	}

	if len(response.Segments) != 1 {
		t.Errorf("ожидался 1 сегмент, получено %d", len(response.Segments))
	}
}
