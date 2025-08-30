-- +goose Up
-- +goose StatementBegin

-- Создание таблицы платежей
CREATE TABLE payments (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount DECIMAL(10,2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'RUB',
    payment_id VARCHAR(255) NOT NULL UNIQUE, -- ID от ЮKassa
    status VARCHAR(50) DEFAULT 'pending',
    premium_duration_days INTEGER NOT NULL, -- Длительность премиума в днях
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE, -- Дата завершения платежа
    metadata JSONB -- Дополнительные данные (план, название и т.д.)
);

-- Создание индексов для оптимизации
CREATE INDEX idx_payments_user_id ON payments(user_id);
CREATE INDEX idx_payments_payment_id ON payments(payment_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_created_at ON payments(created_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS update_payments_updated_at ON payments;
DROP TABLE IF EXISTS payments;

-- +goose StatementEnd
