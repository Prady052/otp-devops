package model

import "time"

type OTPRecord struct {
	Identifier   string
	OTPHash      string
	ExpiresAt    time.Time
	AttemptsLeft int
}
