package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
	"time"

	"otp-devops/backend/internal/model"
	"otp-devops/backend/internal/repository"
)

type OTPService struct {
	repo        *repository.RedisRepo
	ttl         time.Duration
	maxAttempts int
}

func NewOTPService(repo *repository.RedisRepo, ttlMinutes, maxAttempts int) *OTPService {
	return &OTPService{
		repo:        repo,
		ttl:         time.Duration(ttlMinutes) * time.Minute,
		maxAttempts: maxAttempts,
	}
}

// GenerateOTP creates a cryptographically secure numeric OTP of the given length.
func (s *OTPService) GenerateOTP(length int) (string, error) {
	if length < 4 || length > 8 {
		length = 6
	}

	otp := ""
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("failed to generate OTP digit: %w", err)
		}
		otp += fmt.Sprintf("%d", n.Int64())
	}

	return otp, nil
}

// HashOTP returns the SHA-256 hex digest of the OTP string.
func (s *OTPService) HashOTP(otp string) string {
	hash := sha256.Sum256([]byte(otp))
	return fmt.Sprintf("%x", hash)
}

// RequestOTP generates a new OTP for the given identifier.
// Returns the plain-text OTP (for dev display; in production this would be sent via email/SMS).
func (s *OTPService) RequestOTP(ctx context.Context, identifier string, length int) (string, error) {
	// Check rate limit — prevent requesting a new OTP while one is still active
	limited, err := s.repo.IsRateLimited(ctx, identifier)
	if err != nil {
		return "", fmt.Errorf("rate limit check failed: %w", err)
	}
	if limited {
		return "", fmt.Errorf("OTP already sent. Please wait before requesting again")
	}

	// Generate OTP
	otp, err := s.GenerateOTP(length)
	if err != nil {
		return "", err
	}

	// Create record with hashed OTP
	record := model.OTPRecord{
		Identifier:   identifier,
		OTPHash:      s.HashOTP(otp),
		ExpiresAt:    time.Now().Add(s.ttl),
		AttemptsLeft: s.maxAttempts,
	}

	// Store in Redis with TTL
	if err := s.repo.SaveOTP(ctx, identifier, record, s.ttl); err != nil {
		return "", fmt.Errorf("failed to store OTP: %w", err)
	}

	// Set rate limit (60 seconds between requests)
	if err := s.repo.SetRateLimit(ctx, identifier, 60*time.Second); err != nil {
		// Non-critical — log but don't fail
		fmt.Printf("warning: failed to set rate limit for %s: %v\n", identifier, err)
	}

	return otp, nil
}

// VerifyOTP checks the provided OTP against the stored hash.
func (s *OTPService) VerifyOTP(ctx context.Context, identifier, otp string) (bool, error) {
	record, err := s.repo.GetOTP(ctx, identifier)
	if err != nil {
		return false, fmt.Errorf("failed to retrieve OTP: %w", err)
	}
	if record == nil {
		return false, fmt.Errorf("no OTP found for this identifier. Please request a new one")
	}

	// Check remaining attempts
	if record.AttemptsLeft <= 0 {
		_ = s.repo.DeleteOTP(ctx, identifier)
		return false, fmt.Errorf("maximum attempts exceeded. Please request a new OTP")
	}

	// Compare hashes
	if s.HashOTP(otp) == record.OTPHash {
		// OTP is correct — single use, so delete it
		_ = s.repo.DeleteOTP(ctx, identifier)
		return true, nil
	}

	// Wrong OTP — decrement attempts
	record.AttemptsLeft--
	if err := s.repo.UpdateOTP(ctx, identifier, *record); err != nil {
		return false, fmt.Errorf("failed to update attempts: %w", err)
	}

	return false, fmt.Errorf("invalid OTP. %d attempts remaining", record.AttemptsLeft)
}
