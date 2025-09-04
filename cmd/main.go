package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lingua-ai/internal/ai"
	"lingua-ai/internal/bot"
	"lingua-ai/internal/config"
	"lingua-ai/internal/flashcards"
	"lingua-ai/internal/message"
	"lingua-ai/internal/metrics"
	"lingua-ai/internal/migrations"
	"lingua-ai/internal/payment"
	"lingua-ai/internal/premium"
	"lingua-ai/internal/referral"
	"lingua-ai/internal/scheduler"
	"lingua-ai/internal/store"
	"lingua-ai/internal/tts"
	"lingua-ai/internal/user"
	"lingua-ai/internal/webhook"
	"lingua-ai/internal/whisper"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

func main() {
	// Инициализация логгера
	logger, err := initLogger()
	if err != nil {
		fmt.Printf("Ошибка инициализации логгера: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("запуск приложения Lingua AI")

	// Загрузка конфигурации
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("ошибка загрузки конфигурации", zap.Error(err))
	}

	// Инициализация базы данных
	store, err := store.NewStore(cfg, logger)
	if err != nil {
		logger.Fatal("ошибка инициализации базы данных", zap.Error(err))
	}
	defer store.Close()

	// Применение миграций
	if err := migrations.RunMigrations(cfg, logger); err != nil {
		logger.Fatal("ошибка применения миграций", zap.Error(err))
	}

	// Инициализация AI клиента
	logger.Info("конфигурация AI",
		zap.String("provider", cfg.AI.Provider),
		zap.String("model", cfg.AI.Model))

	aiClient, err := ai.NewAIClient(&ai.AIConfig{
		Provider:    cfg.AI.Provider,
		Model:       cfg.AI.Model,
		MaxTokens:   cfg.AI.MaxTokens,
		Temperature: cfg.AI.Temperature,
		DeepSeek: ai.DeepSeekConfig{
			APIKey:  cfg.AI.DeepSeek.APIKey,
			BaseURL: cfg.AI.DeepSeek.BaseURL,
		},
	}, logger)
	if err != nil {
		logger.Fatal("ошибка создания AI клиента", zap.Error(err))
	}

	// Инициализация Whisper клиента
	whisperClient := whisper.NewClient(cfg.Whisper.APIURL, logger)

	// Инициализация TTS сервиса
	var ttsService tts.TTSService
	if cfg.TTS.Enabled {
		ttsService = tts.NewPiperService(logger, cfg.TTS.BaseURL)
		logger.Info("Piper TTS сервис инициализирован")
	} else {
		logger.Info("TTS сервис отключен")
	}

	// Инициализация сервисов
	userService := user.NewService(store, logger)
	messageService := message.NewService(store, logger)
	flashcardService := flashcards.NewService(store.Flashcard(), logger)

	// Инициализация YooKassa клиента
	yukassaClient := payment.NewYukassaClient(cfg.YooKassa.ShopID, cfg.YooKassa.SecretKey, cfg.YooKassa.TestMode, logger)
	logger.Info("YooKassa клиент инициализирован", zap.String("shop_id", cfg.YooKassa.ShopID))

	// Инициализация premium service
	premiumService := premium.NewService(userService, store.Payment(), yukassaClient, logger)

	// Инициализация referral сервиса
	referralService := referral.NewService(store.Referral(), store.User(), logger)

	// Инициализация метрик
	metricsSystem := metrics.New(logger)
	userMetrics := metricsSystem
	aiMetrics := metricsSystem

	// Инициализация HTTP handler для метрик
	metricsHandler := metrics.NewHandler(metricsSystem, logger)

	// Инициализация Telegram бота
	botAPI, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		logger.Fatal("ошибка инициализации Telegram бота", zap.Error(err))
	}

	// Логируем конфигурацию YooKassa для отладки
	logger.Info("конфигурация YooKassa",
		zap.String("shop_id", cfg.YooKassa.ShopID),
		zap.Bool("test_mode", cfg.YooKassa.TestMode))

	botInfo, err := botAPI.GetMe()
	if err != nil {
		logger.Fatal("ошибка получения информации о боте", zap.Error(err))
	}

	logger.Info("Telegram бот инициализирован",
		zap.String("username", botInfo.UserName),
		zap.Int64("id", botInfo.ID))

	// Инициализация обработчика
	handler := bot.NewHandler(botAPI, userService, messageService, aiClient, whisperClient, ttsService, logger, userMetrics, aiMetrics, premiumService, referralService, flashcardService, store)

	// Инициализация планировщика задач
	taskScheduler := scheduler.NewScheduler(logger)

	// Добавляем джобу для неактивных пользователей
	inactiveUsersJob := scheduler.NewInactiveUsersJob(userService, messageService, aiClient, botAPI, logger)
	taskScheduler.AddJob(inactiveUsersJob)

	// Создание канала для graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Обработка сигналов для graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Запуск HTTP сервера для метрик
	go startMetricsServer(ctx, cfg.App.Port, metricsHandler, premiumService, cfg.YooKassa.SecretKey, logger)

	// Запуск планировщика задач (каждые 4 часа)
	go taskScheduler.Start(ctx, 4*time.Hour)

	// Запуск обработки обновлений
	go handleUpdates(ctx, botAPI, handler, logger)

	logger.Info("приложение запущено и готово к работе",
		zap.String("address", fmt.Sprintf("http://localhost:%d", cfg.App.Port)),
	)

	// Ожидание сигнала завершения
	<-sigChan
	logger.Info("получен сигнал завершения, начинаем graceful shutdown")

	// Graceful shutdown
	_, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Останавливаем получение обновлений
	botAPI.StopReceivingUpdates()

	logger.Info("приложение завершено")
}

// initLogger инициализирует логгер
func initLogger() (*zap.Logger, error) {
	// В продакшене можно использовать JSON формат
	config := zap.NewDevelopmentConfig()
	config.OutputPaths = []string{"stdout", "logs/app.log"}
	config.ErrorOutputPaths = []string{"stderr", "logs/error.log"}

	// Создаем директорию для логов если её нет
	if err := os.MkdirAll("logs", 0755); err != nil {
		return nil, fmt.Errorf("ошибка создания директории логов: %w", err)
	}

	return config.Build()
}

// handleUpdates обрабатывает обновления от Telegram
func handleUpdates(ctx context.Context, bot *tgbotapi.BotAPI, handler *bot.Handler, logger *zap.Logger) {
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := bot.GetUpdatesChan(updateConfig)

	for {
		select {
		case update := <-updates:
			// Пропускаем пустые обновления
			if update.Message == nil && update.CallbackQuery == nil {
				continue
			}

			// Обрабатываем обновление в горутине
			go func(update tgbotapi.Update) {
				if err := handler.HandleUpdate(ctx, update); err != nil {
					// Определяем chat_id для логирования
					var chatID int64
					if update.Message != nil {
						chatID = update.Message.Chat.ID
					} else if update.CallbackQuery != nil && update.CallbackQuery.Message != nil {
						chatID = update.CallbackQuery.Message.Chat.ID
					}

					logger.Error("ошибка обработки обновления",
						zap.Int64("chat_id", chatID),
						zap.Error(err))
				}
			}(update)

		case <-ctx.Done():
			logger.Info("остановка обработки обновлений")
			return
		}
	}
}

// startMetricsServer запускает HTTP сервер для метрик и webhook'ов
func startMetricsServer(ctx context.Context, port int, handler *metrics.Handler, premiumService *premium.Service, yukassaSecretKey string, logger *zap.Logger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", handler.MetricsHandler())
	mux.HandleFunc("/health", handler.HealthHandler)

	// Webhook endpoint для ЮKassa
	webhookHandler := webhook.NewYooKassaWebhookHandler(premiumService, yukassaSecretKey, logger)
	mux.HandleFunc("/webhook/yukassa", webhookHandler.HandleWebhook)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	logger.Info("HTTP сервер метрик запущен", zap.String("address", server.Addr))

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("ошибка HTTP сервера метрик", zap.Error(err))
		}
	}()

	// Ожидание сигнала завершения
	<-ctx.Done()

	// Graceful shutdown HTTP сервера
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("ошибка при остановке HTTP сервера метрик", zap.Error(err))
	}

	logger.Info("HTTP сервер метрик остановлен")
}
