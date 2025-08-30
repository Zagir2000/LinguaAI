package metrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Metrics содержит все метрики приложения
type Metrics struct {
	logger *zap.Logger

	// Счетчики
	userLogins   *prometheus.CounterVec
	userMessages *prometheus.CounterVec
	aiRequests   *prometheus.CounterVec
	xpEarned     *prometheus.CounterVec

	// Гистограммы
	aiResponseTime *prometheus.HistogramVec
	xpPerAction    prometheus.Histogram

	// Gauge метрики
	activeUsers   prometheus.Gauge
	lastUserLogin prometheus.Gauge

	// Мьютекс для thread-safety
	mu sync.RWMutex
}

// New создает новый экземпляр метрик
func New(logger *zap.Logger) *Metrics {
	m := &Metrics{
		logger: logger,

		// Счетчики пользователей
		userLogins: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "user_logins_total",
				Help: "Общее количество входов пользователей",
			},
			[]string{"type"}, // daily, total
		),

		// Счетчики сообщений
		userMessages: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "user_messages_total",
				Help: "Общее количество сообщений пользователей",
			},
			[]string{"type"}, // text, voice, daily
		),

		// Счетчики AI запросов
		aiRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ai_requests_total",
				Help: "Общее количество запросов к AI",
			},
			[]string{"type", "status"}, // type: russian_with_translation, english_practice; status: success, failed
		),

		// Счетчики опыта
		xpEarned: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "xp_earned_total",
				Help: "Общее количество заработанного опыта",
			},
			[]string{"source"}, // russian_message, exercise_request, daily_bonus
		),

		// Гистограмма времени ответа AI
		aiResponseTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "ai_response_time_seconds",
				Help:    "Время ответа AI в секундах",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"type"}, // russian_with_translation, english_practice
		),

		// Гистограмма опыта за действие
		xpPerAction: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "xp_per_action",
				Help:    "Количество опыта за одно действие",
				Buckets: []float64{1, 2, 3, 5, 10, 15, 20, 25, 30, 50, 100},
			},
		),

		// Gauge активных пользователей
		activeUsers: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "active_users",
				Help: "Количество активных пользователей",
			},
		),

		// Gauge времени последнего входа
		lastUserLogin: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "last_user_login",
				Help: "Timestamp последнего входа пользователя",
			},
		),
	}

	// Регистрируем все метрики
	prometheus.MustRegister(
		m.userLogins,
		m.userMessages,
		m.aiRequests,
		m.xpEarned,
		m.aiResponseTime,
		m.xpPerAction,
		m.activeUsers,
		m.lastUserLogin,
	)

	return m
}

// IncrementCounter увеличивает счетчик
func (m *Metrics) IncrementCounter(name string, labels ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var counter *prometheus.CounterVec

	switch name {
	case "user_logins_total":
		counter = m.userLogins
	case "user_messages_total":
		counter = m.userMessages
	case "ai_requests_total":
		counter = m.aiRequests
	case "xp_earned_total":
		counter = m.xpEarned
	default:
		m.logger.Error("неизвестная метрика", zap.String("name", name))
		return
	}

	counter.WithLabelValues(labels...).Inc()
	m.logger.Debug("метрика увеличена", zap.String("metric", name), zap.Int("count", len(labels)))
}

// SetGauge устанавливает значение gauge метрики
func (m *Metrics) SetGauge(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var gauge prometheus.Gauge

	switch name {
	case "active_users":
		gauge = m.activeUsers
	case "last_user_login":
		gauge = m.lastUserLogin
	default:
		m.logger.Error("неизвестная gauge метрика", zap.String("name", name))
		return
	}

	gauge.Set(value)
	m.logger.Debug("метрика установлена", zap.String("metric", name), zap.Float64("value", value))
}

// ObserveHistogram добавляет наблюдение в гистограмму
func (m *Metrics) ObserveHistogram(name string, value float64, labels ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch name {
	case "ai_response_time":
		m.aiResponseTime.WithLabelValues(labels...).Observe(value)
	case "xp_per_action":
		m.xpPerAction.Observe(value)
	default:
		m.logger.Error("неизвестная гистограмма", zap.String("name", name))
		return
	}

	m.logger.Debug("гистограмма обновлена", zap.String("metric", name), zap.Float64("value", value))
}

// RecordUserLogin записывает вход пользователя
func (m *Metrics) RecordUserLogin(userID int64) {
	m.IncrementCounter("user_logins_total", "total")
	m.IncrementCounter("user_logins_total", "daily")
	m.SetGauge("last_user_login", float64(userID))
}

// RecordUserMessage записывает сообщение пользователя
func (m *Metrics) RecordUserMessage(messageType string) {
	m.IncrementCounter("user_messages_total", messageType)
	m.IncrementCounter("user_messages_total", "daily")
}

// RecordAIRequest записывает запрос к AI
func (m *Metrics) RecordAIRequest(requestType string, success bool, responseTime float64) {
	status := "success"
	if !success {
		status = "failed"
	}

	m.IncrementCounter("ai_requests_total", requestType, status)

	m.ObserveHistogram("ai_response_time", responseTime, requestType)
}

// RecordXP записывает заработанный опыт
func (m *Metrics) RecordXP(userID int64, amount int, source string) {
	m.IncrementCounter("xp_earned_total", source)
	m.ObserveHistogram("xp_per_action", float64(amount))
}

// Handler возвращает HTTP handler для метрик
func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}
