package model

type RequestOTPInput struct {
	Identifier string `json:"identifier" binding:"required"`
	OTPLength  int    `json:"otp_length"`
}

type VerifyOTPInput struct {
	Identifier string `json:"identifier" binding:"required"`
	OTP        string `json:"otp" binding:"required"`
}
