package config

import (
	"os"
	"strconv"
)

type Config struct {
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	OTPTtlMinutes  int
	OTPMaxAttempts int
	ServerPort     string
}

func LoadConfig() *Config {
	return &Config{
		RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:  getEnv("REDIS_PASSWORD", ""),
		RedisDB:        getEnvInt("REDIS_DB", 0),
		OTPTtlMinutes:  getEnvInt("OTP_TTL_MINUTES", 5),
		OTPMaxAttempts: getEnvInt("OTP_MAX_ATTEMPTS", 5),
		ServerPort:     getEnv("SERVER_PORT", "8080"),
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}
