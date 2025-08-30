-- +goose Up
-- +goose StatementBegin

-- Создание таблицы пользователей
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT UNIQUE NOT NULL,
    username VARCHAR(255),
    first_name VARCHAR(255) NOT NULL,
    last_name VARCHAR(255),
    level VARCHAR(50) NOT NULL DEFAULT 'beginner',
    xp INTEGER NOT NULL DEFAULT 0,
    last_seen TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    current_state VARCHAR(50) DEFAULT 'idle',
    study_streak INTEGER DEFAULT 0,
    last_study_date TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    is_premium BOOLEAN DEFAULT false,
    premium_expires_at TIMESTAMP WITHOUT TIME ZONE,
    messages_count INTEGER DEFAULT 0,
    max_messages INTEGER DEFAULT 15,
    messages_reset_date DATE DEFAULT CURRENT_DATE,
    last_test_date DATE
);

-- Создание индексов для оптимизации
CREATE INDEX idx_users_telegram_id ON users(telegram_id);
CREATE INDEX idx_users_level ON users(level);
CREATE INDEX idx_users_xp ON users(xp);
CREATE INDEX idx_users_study_streak ON users(study_streak);
CREATE INDEX idx_users_current_state ON users(current_state);
CREATE INDEX idx_users_is_premium ON users(is_premium);
CREATE INDEX idx_users_last_seen ON users(last_seen);
CREATE INDEX idx_users_last_study_date ON users(last_study_date);
CREATE INDEX idx_users_messages_reset_date ON users(messages_reset_date);
CREATE INDEX idx_users_premium_expires_at ON users(premium_expires_at);

-- Добавление триггера для обновления updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at 
    BEFORE UPDATE ON users 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP TABLE IF EXISTS users;

-- +goose StatementEnd
