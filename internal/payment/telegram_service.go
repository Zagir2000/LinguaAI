package payment

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// TelegramPaymentService сервис для работы с Telegram Payments API
type TelegramPaymentService struct {
	botToken      string
	providerToken string
	baseURL       string
	httpClient    *http.Client
}

// NewTelegramPaymentService создает новый сервис
func NewTelegramPaymentService(botToken, providerToken string) *TelegramPaymentService {
	return &TelegramPaymentService{
		botToken:      botToken,
		providerToken: providerToken,
		baseURL:       "https://api.telegram.org/bot" + botToken,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

// SendInvoice отправляет счет на оплату
func (s *TelegramPaymentService) SendInvoice(chatID int64, invoice *Invoice) error {
	endpoint := s.baseURL + "/sendInvoice"

	// Логируем создание invoice для отладки
	fmt.Printf("🔧 Создаем invoice для chat_id: %d, payload: %s, provider_token: %s\n",
		chatID, invoice.Payload, s.providerToken)

	// Подготавливаем данные для отправки
	data := url.Values{}
	data.Set("chat_id", strconv.FormatInt(chatID, 10))
	data.Set("title", invoice.Title)
	data.Set("description", invoice.Description)
	data.Set("payload", invoice.Payload)
	data.Set("provider_token", s.providerToken)
	data.Set("currency", invoice.Currency)

	// Добавляем цены
	pricesJSON, err := json.Marshal(invoice.Prices)
	if err != nil {
		return fmt.Errorf("ошибка маршалинга цен: %w", err)
	}
	data.Set("prices", string(pricesJSON))

	// Добавляем опциональные параметры
	if invoice.MaxTipAmount != nil {
		data.Set("max_tip_amount", strconv.Itoa(*invoice.MaxTipAmount))
	}
	if len(invoice.SuggestedTipAmounts) > 0 {
		tipsJSON, err := json.Marshal(invoice.SuggestedTipAmounts)
		if err != nil {
			return fmt.Errorf("ошибка маршалинга чаевых: %w", err)
		}
		data.Set("suggested_tip_amounts", string(tipsJSON))
	}
	if invoice.StartParameter != "" {
		data.Set("start_parameter", invoice.StartParameter)
	}
	if invoice.PhotoURL != "" {
		data.Set("photo_url", invoice.PhotoURL)
	}
	if invoice.PhotoSize != nil {
		data.Set("photo_size", strconv.Itoa(*invoice.PhotoSize))
	}
	if invoice.PhotoWidth != nil {
		data.Set("photo_width", strconv.Itoa(*invoice.PhotoWidth))
	}
	if invoice.PhotoHeight != nil {
		data.Set("photo_height", strconv.Itoa(*invoice.PhotoHeight))
	}

	// Булевы параметры
	if invoice.NeedName {
		data.Set("need_name", "true")
	}
	if invoice.NeedPhoneNumber {
		data.Set("need_phone_number", "true")
	}
	if invoice.NeedEmail {
		data.Set("need_email", "true")
	}
	if invoice.NeedShippingAddress {
		data.Set("need_shipping_address", "true")
	}
	if invoice.SendPhoneNumberToProvider {
		data.Set("send_phone_number_to_provider", "true")
	}
	if invoice.SendEmailToProvider {
		data.Set("send_email_to_provider", "true")
	}
	if invoice.IsFlexible {
		data.Set("is_flexible", "true")
	}

	// Отправляем запрос
	resp, err := s.httpClient.PostForm(endpoint, data)
	if err != nil {
		return fmt.Errorf("ошибка отправки запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем ответ
	if resp.StatusCode != http.StatusOK {
		// Читаем тело ответа для отладки
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("неуспешный статус: %d, ответ: %s", resp.StatusCode, string(body))
	}

	return nil
}

// AnswerShippingQuery отвечает на запрос о доставке
func (s *TelegramPaymentService) AnswerShippingQuery(shippingQueryID string, ok bool, shippingOptions []ShippingOption, errorMessage string) error {
	endpoint := s.baseURL + "/answerShippingQuery"

	data := url.Values{}
	data.Set("shipping_query_id", shippingQueryID)
	data.Set("ok", strconv.FormatBool(ok))

	if ok && len(shippingOptions) > 0 {
		optionsJSON, err := json.Marshal(shippingOptions)
		if err != nil {
			return fmt.Errorf("ошибка маршалинга вариантов доставки: %w", err)
		}
		data.Set("shipping_options", string(optionsJSON))
	} else if !ok && errorMessage != "" {
		data.Set("error_message", errorMessage)
	}

	// Отправляем запрос
	resp, err := s.httpClient.PostForm(endpoint, data)
	if err != nil {
		return fmt.Errorf("ошибка отправки запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("неуспешный статус: %d", resp.StatusCode)
	}

	return nil
}

// AnswerPreCheckoutQuery отвечает на предварительную проверку платежа
func (s *TelegramPaymentService) AnswerPreCheckoutQuery(preCheckoutQueryID string, ok bool, errorMessage string) error {
	endpoint := s.baseURL + "/answerPreCheckoutQuery"

	data := url.Values{}
	data.Set("pre_checkout_query_id", preCheckoutQueryID)
	data.Set("ok", strconv.FormatBool(ok))

	if !ok && errorMessage != "" {
		data.Set("error_message", errorMessage)
	}

	// Отправляем запрос
	resp, err := s.httpClient.PostForm(endpoint, data)
	if err != nil {
		return fmt.Errorf("ошибка отправки запроса: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("неуспешный статус: %d", resp.StatusCode)
	}

	return nil
}

// CreatePremiumInvoice создает счет для премиум подписки
func (s *TelegramPaymentService) CreatePremiumInvoice(userID int64, plan string, durationDays int, amountKopecks int) *Invoice {
	payload := fmt.Sprintf("premium_%d_%s_%d", userID, plan, durationDays)

	// Логируем создание invoice
	fmt.Printf("💳 Создаем invoice: user_id=%d, plan=%s, duration=%d дней, amount=%d копеек\n",
		userID, plan, durationDays, amountKopecks)

	invoice := &Invoice{
		Title:         "🌟 Премиум подписка Lingua AI",
		Description:   fmt.Sprintf("Премиум подписка на %d дней", durationDays),
		Payload:       payload,
		ProviderToken: s.providerToken,
		Currency:      "RUB",
		Prices: []LabeledPrice{
			{
				Label:  fmt.Sprintf("Премиум подписка (%d дней)", durationDays),
				Amount: amountKopecks,
			},
		},
		StartParameter: "premium",
		PhotoURL:       "https://lingua-ai.ru/images/premium.jpg",
		NeedEmail:      true,
		IsFlexible:     false,
	}

	fmt.Printf("✅ Invoice создан: payload=%s, provider_token=%s\n", payload, s.providerToken)
	return invoice
}

// CreateShippingOptions создает варианты доставки
func (s *TelegramPaymentService) CreateShippingOptions() []ShippingOption {
	return []ShippingOption{
		{
			ID:    "digital",
			Title: "Цифровая доставка",
			Prices: []LabeledPrice{
				{
					Label:  "Цифровая доставка",
					Amount: 0, // Бесплатно для цифровых товаров
				},
			},
		},
	}
}
