-- +goose Up
-- +goose StatementBegin

       -- Добавление полей для реферальной системы в таблицу users
       ALTER TABLE users ADD COLUMN IF NOT EXISTS referral_code VARCHAR(20) UNIQUE NULL;
       ALTER TABLE users ADD COLUMN IF NOT EXISTS referral_count INTEGER DEFAULT 0;
       ALTER TABLE users ADD COLUMN IF NOT EXISTS referred_by BIGINT REFERENCES users(id);

-- Создание таблицы рефералов
CREATE TABLE IF NOT EXISTS referrals (
    id BIGSERIAL PRIMARY KEY,
    referrer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    referred_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'cancelled')),
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(referred_id) -- Один пользователь может быть приглашен только один раз
);

-- Создание индексов для оптимизации
CREATE INDEX IF NOT EXISTS idx_users_referral_code ON users(referral_code);
CREATE INDEX IF NOT EXISTS idx_users_referral_count ON users(referral_count);
CREATE INDEX IF NOT EXISTS idx_users_referred_by ON users(referred_by);

CREATE INDEX IF NOT EXISTS idx_referrals_referrer_id ON referrals(referrer_id);
CREATE INDEX IF NOT EXISTS idx_referrals_referred_id ON referrals(referred_id);
CREATE INDEX IF NOT EXISTS idx_referrals_status ON referrals(status);
CREATE INDEX IF NOT EXISTS idx_referrals_created_at ON referrals(created_at);

-- Создание функции для генерации уникального реферального кода
CREATE OR REPLACE FUNCTION generate_referral_code() 
RETURNS VARCHAR(20) AS $$
DECLARE
    new_code VARCHAR(20);
    attempts INTEGER := 0;
    max_attempts INTEGER := 10;
BEGIN
    LOOP
        -- Генерируем код из 8 символов (буквы и цифры)
        new_code := upper(substring(md5(random()::text || clock_timestamp()::text) from 1 for 8));
        
        -- Проверяем уникальность
        IF NOT EXISTS (SELECT 1 FROM users WHERE referral_code = new_code) THEN
            RETURN new_code;
        END IF;
        
        attempts := attempts + 1;
        IF attempts >= max_attempts THEN
            RAISE EXCEPTION 'Не удалось сгенерировать уникальный реферальный код';
        END IF;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Создание функции для проверки и начисления премиума за рефералы
CREATE OR REPLACE FUNCTION check_referral_premium(user_id BIGINT) 
RETURNS BOOLEAN AS $$
DECLARE
    referral_count INTEGER;
    current_premium BOOLEAN;
BEGIN
    -- Получаем количество завершенных рефералов
    SELECT COUNT(*) INTO referral_count
    FROM referrals 
    WHERE referrer_id = user_id AND status = 'completed';
    
    -- Если достигли 10 рефералов
    IF referral_count >= 10 THEN
        -- Обновляем статус пользователя
        UPDATE users 
        SET 
            is_premium = true,
            premium_expires_at = CASE 
                WHEN premium_expires_at IS NULL OR premium_expires_at < NOW() 
                THEN NOW() + INTERVAL '1 month'
                ELSE premium_expires_at + INTERVAL '1 month'
            END
        WHERE id = user_id;
        
        RETURN true;
    END IF;
    
    RETURN false;
END;
$$ LANGUAGE plpgsql;

-- Создание триггера для автоматической проверки премиума при изменении статуса реферала
CREATE OR REPLACE FUNCTION trigger_check_referral_premium() 
RETURNS TRIGGER AS $$
BEGIN
    -- Если статус реферала изменился на 'completed'
    IF NEW.status = 'completed' AND (OLD.status IS NULL OR OLD.status != 'completed') THEN
        -- Проверяем, нужно ли начислить премиум
        PERFORM check_referral_premium(NEW.referrer_id);
        
        -- Обновляем счетчик рефералов у приглашающего
        UPDATE users 
        SET referral_count = (
            SELECT COUNT(*) 
            FROM referrals 
            WHERE referrer_id = NEW.referrer_id AND status = 'completed'
        )
        WHERE id = NEW.referrer_id;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_referral_premium_check
    AFTER INSERT OR UPDATE ON referrals
    FOR EACH ROW
    EXECUTE FUNCTION trigger_check_referral_premium();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trigger_referral_premium_check ON referrals;
DROP FUNCTION IF EXISTS trigger_check_referral_premium();
DROP FUNCTION IF EXISTS check_referral_premium(BIGINT);
DROP FUNCTION IF EXISTS generate_referral_code();

DROP TABLE IF EXISTS referrals;

ALTER TABLE users DROP COLUMN IF EXISTS referred_by;
ALTER TABLE users DROP COLUMN IF EXISTS referral_count;
ALTER TABLE users DROP COLUMN IF EXISTS referral_code;

-- +goose StatementEnd
