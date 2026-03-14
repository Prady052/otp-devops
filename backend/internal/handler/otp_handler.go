package handler

import (
	"net/http"

	"otp-devops/backend/internal/model"
	"otp-devops/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	otpService *service.OTPService
}

func NewHandler(otpService *service.OTPService) *Handler {
	return &Handler{otpService: otpService}
}

func (h *Handler) RequestOTPHandler(c *gin.Context) {
	var input model.RequestOTPInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request. 'identifier' is required.",
		})
		return
	}

	// Default OTP length to 6 if not provided or out of range
	if input.OTPLength < 4 || input.OTPLength > 8 {
		input.OTPLength = 6
	}

	otp, err := h.otpService.RequestOTP(c.Request.Context(), input.Identifier, input.OTPLength)
	if err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// In production, the OTP would be sent via email/SMS and NOT returned in the response.
	// For development/demo purposes, we return it so the frontend can display it.
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "OTP sent successfully",
		"otp":     otp,
	})
}

func (h *Handler) VerifyOTPHandler(c *gin.Context) {
	var input model.VerifyOTPInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request. 'identifier' and 'otp' are required.",
		})
		return
	}

	valid, err := h.otpService.VerifyOTP(c.Request.Context(), input.Identifier, input.OTP)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"error":   "Invalid OTP",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "OTP verified successfully",
	})
}
