package main

import (
	"fmt"
	"log"

	"otp-devops/backend/internal/config"
	"otp-devops/backend/internal/handler"
	"otp-devops/backend/internal/repository"
	"otp-devops/backend/internal/service"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Connect to Redis
	repo, err := repository.NewRedisRepo(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer repo.Close()
	log.Println("Connected to Redis successfully")

	// Create service and handler
	otpService := service.NewOTPService(repo, cfg.OTPTtlMinutes, cfg.OTPMaxAttempts)
	h := handler.NewHandler(otpService)

	// Setup router and start server
	router := handler.SetupRouter(h)

	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("Server starting on %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
