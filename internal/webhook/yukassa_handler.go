package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"lingua-ai/internal/premium"

	"go.uber.org/zap"
)

// YooKassaWebhookHandler обрабатывает webhook'и от ЮKassa
type YooKassaWebhookHandler struct {
	premiumService *premium.Service
	logger         *zap.Logger
	secretKey      string
}

// NewYooKassaWebhookHandler создает новый обработчик webhook'ов
func NewYooKassaWebhookHandler(premiumService *premium.Service, secretKey string, logger *zap.Logger) *YooKassaWebhookHandler {
	return &YooKassaWebhookHandler{
		premiumService: premiumService,
		logger:         logger,
		secretKey:      secretKey,
	}
}

// PaymentWebhook представляет webhook от ЮKassa
type PaymentWebhook struct {
	Type   string `json:"type"`
	Event  string `json:"event"`
	Object struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Amount struct {
			Value    string `json:"value"`
			Currency string `json:"currency"`
		} `json:"amount"`
		Metadata map[string]string `json:"metadata"`
	} `json:"object"`
}

// HandleWebhook обрабатывает входящий webhook от ЮKassa
func (h *YooKassaWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("получен webhook запрос",
		zap.String("method", r.Method),
		zap.String("url", r.URL.String()),
		zap.String("user_agent", r.UserAgent()),
		zap.String("content_type", r.Header.Get("Content-Type")))

	// Проверяем метод запроса
	if r.Method != http.MethodPost {
		h.logger.Warn("неверный метод webhook запроса", zap.String("method", r.Method))
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Читаем тело запроса
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("ошибка чтения тела запроса", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	// Логируем тело webhook'а для отладки
	h.logger.Info("тело webhook'а",
		zap.String("body", string(body)),
		zap.Int("body_length", len(body)))

	// Проверяем подпись webhook'а (если настроена)
	if !h.verifySignature(r.Header.Get("X-YooKassa-Signature"), body) {
		h.logger.Warn("неверная подпись webhook'а")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Парсим webhook
	var webhook PaymentWebhook
	if err := json.Unmarshal(body, &webhook); err != nil {
		h.logger.Error("ошибка парсинга webhook'а", zap.Error(err))
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	h.logger.Info("получен webhook от ЮKassa",
		zap.String("type", webhook.Type),
		zap.String("event", webhook.Event),
		zap.String("payment_id", webhook.Object.ID),
		zap.String("status", webhook.Object.Status))

	// Обрабатываем webhook в зависимости от типа события
	switch webhook.Event {
	case "payment.succeeded":
		if err := h.handlePaymentSucceeded(context.Background(), webhook); err != nil {
			h.logger.Error("ошибка обработки успешного платежа", zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	case "payment.canceled":
		if err := h.handlePaymentCanceled(context.Background(), webhook); err != nil {
			h.logger.Error("ошибка обработки отмененного платежа", zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	default:
		h.logger.Info("неизвестное событие webhook'а", zap.String("event", webhook.Event))
	}

	// Отвечаем успехом
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handlePaymentSucceeded обрабатывает успешный платеж
func (h *YooKassaWebhookHandler) handlePaymentSucceeded(ctx context.Context, webhook PaymentWebhook) error {
	// Получаем payment_id из webhook'а
	paymentID := webhook.Object.ID

	// Обрабатываем успешный платеж через premium сервис
	if err := h.premiumService.ProcessPaymentCallback(ctx, paymentID, "succeeded"); err != nil {
		return fmt.Errorf("ошибка обработки платежа: %w", err)
	}

	h.logger.Info("платеж успешно обработан",
		zap.String("payment_id", paymentID),
		zap.String("status", "succeeded"))

	return nil
}

// handlePaymentCanceled обрабатывает отмененный платеж
func (h *YooKassaWebhookHandler) handlePaymentCanceled(ctx context.Context, webhook PaymentWebhook) error {
	paymentID := webhook.Object.ID

	// Обрабатываем отмененный платеж
	if err := h.premiumService.ProcessPaymentCallback(ctx, paymentID, "canceled"); err != nil {
		return fmt.Errorf("ошибка обработки отмененного платежа: %w", err)
	}

	h.logger.Info("платеж отменен",
		zap.String("payment_id", paymentID),
		zap.String("status", "canceled"))

	return nil
}

// verifySignature проверяет подпись webhook'а
func (h *YooKassaWebhookHandler) verifySignature(signature string, body []byte) bool {
	if h.secretKey == "" || signature == "" {
		// Если секретный ключ не настроен, пропускаем проверку
		return true
	}

	// Создаем HMAC подпись
	h256 := hmac.New(sha256.New, []byte(h.secretKey))
	h256.Write(body)
	expectedSignature := hex.EncodeToString(h256.Sum(nil))

	// Сравниваем подписи
	return signature == expectedSignature
}
