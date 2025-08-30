-- +goose Up
-- +goose StatementBegin

-- Миграция для оптимизации очистки старых сообщений
-- Добавляем составной индекс для эффективной очистки по пользователю и времени

-- Создание составного индекса для оптимизации операций очистки
CREATE INDEX IF NOT EXISTS idx_user_messages_user_created_desc 
ON user_messages(user_id, created_at DESC);

-- Создание функции для автоматической очистки старых сообщений
CREATE OR REPLACE FUNCTION cleanup_old_messages(
    target_user_id BIGINT, 
    keep_count INTEGER DEFAULT 10
) RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER := 0;
BEGIN
    -- Удаляем старые сообщения, оставляя только последние keep_count
    DELETE FROM user_messages 
    WHERE id IN (
        SELECT id FROM (
            SELECT id, 
                   ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC) as rn
            FROM user_messages 
            WHERE user_id = target_user_id
        ) ranked
        WHERE rn > keep_count
    );
    
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    
    -- Логируем результат
    IF deleted_count > 0 THEN
        RAISE NOTICE 'Удалено % старых сообщений для пользователя %', deleted_count, target_user_id;
    END IF;
    
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- Создание функции для массовой очистки всех пользователей
CREATE OR REPLACE FUNCTION cleanup_all_users_messages(keep_count INTEGER DEFAULT 10) 
RETURNS TABLE(user_id BIGINT, deleted_count INTEGER) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        u.id as user_id,
        cleanup_old_messages(u.id, keep_count) as deleted_count
    FROM users u
    WHERE EXISTS (
        SELECT 1 FROM user_messages um 
        WHERE um.user_id = u.id
    );
END;
$$ LANGUAGE plpgsql;

-- Создание триггерной функции для автоматической очистки при превышении лимита
CREATE OR REPLACE FUNCTION auto_cleanup_messages() RETURNS TRIGGER AS $$
DECLARE
    message_count INTEGER;
    max_messages INTEGER := 10;
BEGIN
    -- Подсчитываем количество сообщений пользователя
    SELECT COUNT(*) INTO message_count 
    FROM user_messages 
    WHERE user_id = NEW.user_id;
    
    -- Если превышен лимит - очищаем старые сообщения
    IF message_count > max_messages THEN
        PERFORM cleanup_old_messages(NEW.user_id, max_messages);
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Создание триггера для автоматической очистки (опционально)
-- Раскомментируйте следующие строки, если хотите автоматическую очистку на уровне БД
-- CREATE TRIGGER trigger_auto_cleanup_messages
--     AFTER INSERT ON user_messages
--     FOR EACH ROW
--     EXECUTE FUNCTION auto_cleanup_messages();

-- Добавление комментариев (с проверкой существования)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_indexes 
        WHERE indexname = 'idx_user_messages_user_created_desc' 
        AND tablename = 'user_messages'
    ) THEN
        COMMENT ON INDEX idx_user_messages_user_created_desc IS 'Составной индекс для оптимизации очистки старых сообщений';
    END IF;
    
    IF NOT EXISTS (
        SELECT 1 FROM pg_proc 
        WHERE proname = 'cleanup_old_messages' 
        AND pronargs = 2
    ) THEN
        COMMENT ON FUNCTION cleanup_old_messages(BIGINT, INTEGER) IS 'Функция для очистки старых сообщений пользователя';
    END IF;
    
    IF NOT EXISTS (
        SELECT 1 FROM pg_proc 
        WHERE proname = 'cleanup_all_users_messages' 
        AND pronargs = 1
    ) THEN
        COMMENT ON FUNCTION cleanup_all_users_messages(INTEGER) IS 'Функция для массовой очистки сообщений всех пользователей';
    END IF;
    
    IF NOT EXISTS (
        SELECT 1 FROM pg_proc 
        WHERE proname = 'auto_cleanup_messages' 
        AND pronargs = 0
    ) THEN
        COMMENT ON FUNCTION auto_cleanup_messages() IS 'Триггерная функция для автоматической очистки при превышении лимита';
    END IF;
END $$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Удаление функций и индексов
DROP FUNCTION IF EXISTS auto_cleanup_messages() CASCADE;
DROP FUNCTION IF EXISTS cleanup_all_users_messages(INTEGER) CASCADE;
DROP FUNCTION IF EXISTS cleanup_old_messages(BIGINT, INTEGER) CASCADE;
DROP INDEX IF EXISTS idx_user_messages_user_created_desc;

-- +goose StatementEnd 