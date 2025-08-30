-- +goose Up
-- +goose StatementBegin

-- Создание таблицы сообщений пользователей
CREATE TABLE user_messages (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL DEFAULT 'user', -- "user" или "assistant"
    content TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Создание индексов для оптимизации
CREATE INDEX idx_user_messages_user_id ON user_messages(user_id);
CREATE INDEX idx_user_messages_created_at ON user_messages(created_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS user_messages;

-- +goose StatementEnd
