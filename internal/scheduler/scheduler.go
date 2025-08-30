package scheduler

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// Scheduler управляет запуском периодических задач
type Scheduler struct {
	logger *zap.Logger
	jobs   []Job
}

// Job интерфейс для периодических задач
type Job interface {
	Run(ctx context.Context) error
}

// NewScheduler создает новый планировщик задач
func NewScheduler(logger *zap.Logger) *Scheduler {
	return &Scheduler{
		logger: logger,
		jobs:   make([]Job, 0),
	}
}

// AddJob добавляет задачу в планировщик
func (s *Scheduler) AddJob(job Job) {
	s.jobs = append(s.jobs, job)
}

// Start запускает планировщик с указанным интервалом
func (s *Scheduler) Start(ctx context.Context, interval time.Duration) {
	s.logger.Info("запуск планировщика задач",
		zap.Duration("interval", interval),
		zap.Int("jobs_count", len(s.jobs)))

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Запускаем задачи сразу при старте
	s.runJobs(ctx)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("остановка планировщика задач")
			return
		case <-ticker.C:
			s.runJobs(ctx)
		}
	}
}

// runJobs запускает все зарегистрированные задачи
func (s *Scheduler) runJobs(ctx context.Context) {
	for i, job := range s.jobs {
		s.logger.Debug("запуск задачи", zap.Int("job_index", i))

		if err := job.Run(ctx); err != nil {
			s.logger.Error("ошибка выполнения задачи",
				zap.Error(err),
				zap.Int("job_index", i))
		}
	}
}
