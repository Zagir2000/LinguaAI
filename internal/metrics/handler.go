package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Handler обрабатывает HTTP запросы для метрик
type Handler struct {
	metrics *Metrics
	logger  *zap.Logger
}

// NewHandler создает новый обработчик метрик
func NewHandler(metrics *Metrics, logger *zap.Logger) *Handler {
	return &Handler{
		metrics: metrics,
		logger:  logger,
	}
}

// MetricsHandler возвращает HTTP handler для Prometheus метрик
func (h *Handler) MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// HealthHandler возвращает статус здоровья сервиса
func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","service":"lingua-ai"}`))
}
