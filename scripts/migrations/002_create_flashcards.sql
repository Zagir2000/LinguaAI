-- +goose Up
-- +goose StatementBegin

-- Создание таблицы карточек для запоминания
CREATE TABLE IF NOT EXISTS flashcards (
    id BIGSERIAL PRIMARY KEY,
    word VARCHAR(255) NOT NULL,
    translation VARCHAR(500) NOT NULL,
    example TEXT,
    level VARCHAR(20) NOT NULL DEFAULT 'beginner',
    category VARCHAR(50) NOT NULL DEFAULT 'general',
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Создание таблицы пользовательских карточек
CREATE TABLE IF NOT EXISTS user_flashcards (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    flashcard_id BIGINT NOT NULL REFERENCES flashcards(id) ON DELETE CASCADE,
    difficulty INTEGER NOT NULL DEFAULT 0,
    review_count INTEGER NOT NULL DEFAULT 0,
    correct_count INTEGER NOT NULL DEFAULT 0,
    last_reviewed_at TIMESTAMP WITHOUT TIME ZONE,
    next_review_at TIMESTAMP WITHOUT TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    is_learned BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, flashcard_id)
);

-- Создание индексов для оптимизации
CREATE INDEX IF NOT EXISTS idx_flashcards_word ON flashcards(word);
CREATE INDEX IF NOT EXISTS idx_flashcards_level ON flashcards(level);
CREATE INDEX IF NOT EXISTS idx_flashcards_category ON flashcards(category);

CREATE INDEX IF NOT EXISTS idx_user_flashcards_user_id ON user_flashcards(user_id);
CREATE INDEX IF NOT EXISTS idx_user_flashcards_is_learned ON user_flashcards(is_learned);
CREATE INDEX IF NOT EXISTS idx_user_flashcards_next_review ON user_flashcards(next_review_at);
CREATE INDEX IF NOT EXISTS idx_user_flashcards_user_next_review ON user_flashcards(user_id, next_review_at);

-- Добавление ограничений для карточек
ALTER TABLE flashcards ADD CONSTRAINT IF NOT EXISTS chk_flashcard_level 
    CHECK (level IN ('beginner', 'intermediate', 'advanced'));

ALTER TABLE flashcards ADD CONSTRAINT IF NOT EXISTS chk_flashcard_category 
    CHECK (category IN ('general', 'business', 'travel', 'food', 'technology', 'education', 'health'));

ALTER TABLE user_flashcards ADD CONSTRAINT IF NOT EXISTS chk_difficulty 
    CHECK (difficulty >= 0 AND difficulty <= 5);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE user_flashcards DROP CONSTRAINT IF EXISTS chk_difficulty;
ALTER TABLE flashcards DROP CONSTRAINT IF EXISTS chk_flashcard_category;
ALTER TABLE flashcards DROP CONSTRAINT IF EXISTS chk_flashcard_level;

DROP TABLE IF EXISTS user_flashcards;
DROP TABLE IF EXISTS flashcards;

-- +goose StatementEnd
