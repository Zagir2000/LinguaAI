package models

import (
	"time"
)

// Referral представляет реферальную связь между пользователями
type Referral struct {
	ID          int64      `json:"id" db:"id"`
	ReferrerID  int64      `json:"referrer_id" db:"referrer_id"`
	ReferredID  int64      `json:"referred_id" db:"referred_id"`
	Status      string     `json:"status" db:"status"`
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
}

// ReferralStatus представляет статус реферала
type ReferralStatus string

const (
	ReferralStatusPending   ReferralStatus = "pending"
	ReferralStatusCompleted ReferralStatus = "completed"
	ReferralStatusCancelled ReferralStatus = "cancelled"
)

// IsValidReferralStatus проверяет валидность статуса реферала
func (rs ReferralStatus) IsValid() bool {
	switch rs {
	case ReferralStatusPending, ReferralStatusCompleted, ReferralStatusCancelled:
		return true
	default:
		return false
	}
}

// ReferralStats представляет статистику рефералов пользователя
type ReferralStats struct {
	TotalReferrals     int `json:"total_referrals"`
	CompletedReferrals int `json:"completed_referrals"`
	PendingReferrals   int `json:"pending_referrals"`
	ReferralsToPremium int `json:"referrals_to_premium"`
}

// ReferralRequest представляет запрос на создание реферала
type ReferralRequest struct {
	ReferrerID int64 `json:"referrer_id"`
	ReferredID int64 `json:"referred_id"`
}

// ReferralUpdateRequest представляет запрос на обновление реферала
type ReferralUpdateRequest struct {
	Status      *string    `json:"status,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}
