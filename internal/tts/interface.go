package tts

import "context"

// TTSService представляет интерфейс для Text-to-Speech сервиса
type TTSService interface {
	// SynthesizeText преобразует текст в аудио
	SynthesizeText(ctx context.Context, text string) ([]byte, error)
}
