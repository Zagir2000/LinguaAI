package payment

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// YukassaClient представляет клиент для работы с ЮKassa API
type YukassaClient struct {
	shopID     string
	secretKey  string
	baseURL    string
	testMode   bool
	httpClient *http.Client
	logger     *zap.Logger
}

// PaymentRequest представляет запрос на создание платежа
type PaymentRequest struct {
	Amount       Amount            `json:"amount"`
	Confirmation Confirmation      `json:"confirmation"`
	Capture      bool              `json:"capture"`
	Description  string            `json:"description"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Amount представляет сумму платежа
type Amount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

// Confirmation представляет способ подтверждения платежа
type Confirmation struct {
	Type            string `json:"type"`
	ReturnURL       string `json:"return_url"`
	ConfirmationURL string `json:"confirmation_url"`
}

// PaymentResponse представляет ответ от ЮKassa
type PaymentResponse struct {
	ID           string            `json:"id"`
	Status       string            `json:"status"`
	Amount       Amount            `json:"amount"`
	Confirmation Confirmation      `json:"confirmation"`
	CreatedAt    string            `json:"created_at"`
	Description  string            `json:"description"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// NewYukassaClient создает новый клиент ЮKassa
func NewYukassaClient(shopID, secretKey string, testMode bool, logger *zap.Logger) *YukassaClient {

	baseURL := "https://api.yookassa.ru/v3"
	if testMode {
		baseURL = "https://api.yookassa.ru/v3" // В тестовом режиме используем тот же URL, но с тестовыми данными
	}

	return &YukassaClient{
		shopID:    shopID,
		secretKey: secretKey,
		baseURL:   baseURL,
		testMode:  testMode,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// CreatePayment создает новый платеж в ЮKassa
func (c *YukassaClient) CreatePayment(ctx context.Context, amount float64, currency string, description string) (string, string, error) {
	// В тестовом режиме возвращаем тестовый ID платежа
	if c.testMode {
		testPaymentID := fmt.Sprintf("test_payment_%d", time.Now().Unix())
		// Создаем рабочую тестовую ссылку для демонстрации
		testURL := fmt.Sprintf("https://yoomoney.ru/checkout/payments/v2/contract?orderId=%s&amount=%.2f&currency=%s",
			testPaymentID, amount, currency)
		c.logger.Info("создан тестовый платеж",
			zap.String("payment_id", testPaymentID),
			zap.String("confirmation_url", testURL),
			zap.Float64("amount", amount),
			zap.String("currency", currency),
			zap.Bool("test_mode", true))
		return testPaymentID, testURL, nil
	}

	// Форматируем сумму (ЮKassa требует строку с двумя знаками после запятой)
	amountStr := fmt.Sprintf("%.2f", amount)

	// Создаем уникальный return URL для этого платежа
	returnURL := fmt.Sprintf("https://t.me/your_bot_username?start=payment_%d", time.Now().Unix())

	paymentReq := PaymentRequest{
		Amount: Amount{
			Value:    amountStr,
			Currency: currency,
		},
		Confirmation: Confirmation{
			Type:      "redirect",
			ReturnURL: returnURL,
		},
		Capture:     true,
		Description: description,
		Metadata: map[string]string{
			"created_at": time.Now().Format(time.RFC3339),
		},
	}

	// Сериализуем запрос
	reqBody, err := json.Marshal(paymentReq)
	if err != nil {
		return "", "", fmt.Errorf("ошибка сериализации запроса: %w", err)
	}

	// Создаем HTTP запрос
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/payments", bytes.NewReader(reqBody))
	if err != nil {
		return "", "", fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	// Устанавливаем заголовки
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+c.getAuthHeader())
	req.Header.Set("Idempotence-Key", fmt.Sprintf("payment_%d", time.Now().UnixNano()))

	// Отправляем запрос
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("ошибка отправки запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("неожиданный статус ответа: %d", resp.StatusCode)
	}

	// Парсим ответ
	var paymentResp PaymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&paymentResp); err != nil {
		return "", "", fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	c.logger.Info("платеж создан в ЮKassa",
		zap.String("payment_id", paymentResp.ID),
		zap.String("amount", paymentResp.Amount.Value),
		zap.String("currency", paymentResp.Amount.Currency),
		zap.String("confirmation_url", paymentResp.Confirmation.ConfirmationURL),
		zap.String("return_url", paymentResp.Confirmation.ReturnURL))

	return paymentResp.ID, paymentResp.Confirmation.ConfirmationURL, nil
}

// CheckPaymentStatus проверяет статус платежа
func (c *YukassaClient) CheckPaymentStatus(ctx context.Context, paymentID string) (string, error) {
	// В тестовом режиме возвращаем успешный статус для тестовых платежей
	if c.testMode && strings.HasPrefix(paymentID, "test_payment_") {
		c.logger.Info("проверка статуса тестового платежа",
			zap.String("payment_id", paymentID),
			zap.String("status", "succeeded"),
			zap.Bool("test_mode", true))
		return "succeeded", nil
	}

	// Создаем HTTP запрос для получения статуса
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/payments/"+paymentID, nil)
	if err != nil {
		return "", fmt.Errorf("ошибка создания HTTP запроса: %w", err)
	}

	// Устанавливаем заголовки
	req.Header.Set("Authorization", "Basic "+c.getAuthHeader())

	// Отправляем запрос
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка отправки запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("неожиданный статус ответа: %d", resp.StatusCode)
	}

	// Парсим ответ
	var paymentResp PaymentResponse
	if err := json.NewDecoder(resp.Body).Decode(&paymentResp); err != nil {
		return "", fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	c.logger.Info("статус платежа получен",
		zap.String("payment_id", paymentID),
		zap.String("status", paymentResp.Status))

	return paymentResp.Status, nil
}

// getAuthHeader создает заголовок авторизации для ЮKassa
func (c *YukassaClient) getAuthHeader() string {
	auth := c.shopID + ":" + c.secretKey
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

// ProcessWebhook обрабатывает webhook от ЮKassa
func (c *YukassaClient) ProcessWebhook(webhookData []byte) (*PaymentWebhook, error) {
	var webhook PaymentWebhook
	if err := json.Unmarshal(webhookData, &webhook); err != nil {
		return nil, fmt.Errorf("ошибка парсинга webhook: %w", err)
	}

	// Проверяем подпись webhook (в реальном приложении)
	// if !c.verifyWebhookSignature(webhookData, signature) {
	//     return nil, fmt.Errorf("неверная подпись webhook")
	// }

	return &webhook, nil
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
