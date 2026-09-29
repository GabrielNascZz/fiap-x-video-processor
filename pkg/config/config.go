package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port            string
	DBHost          string
	DBPort          string
	DBUser          string
	DBPassword      string
	DBName          string
	DBSSLMode       string
	RabbitMQURL     string
	JWTSecret       string
	StorageUploads  string
	StorageOutputs  string
	StorageTemp     string
	Environment     string
}

func LoadConfig() *Config {
	_ = godotenv.Load()

	return &Config{
		Port:           getEnv("PORT", "8080"),
		DBHost:         getEnv("DB_HOST", "postgres"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("DB_USER", "postgres"),
		DBPassword:     getEnv("DB_PASSWORD", "postgres123"),
		DBName:         getEnv("DB_NAME", "fiapx_video_db"),
		DBSSLMode:      getEnv("DB_SSLMODE", "disable"),
		RabbitMQURL:    getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"),
		JWTSecret:      getEnv("JWT_SECRET", "super-secret-fiapx-jwt-key-2026"),
		StorageUploads: getEnv("STORAGE_UPLOADS", "./storage/uploads"),
		StorageOutputs: getEnv("STORAGE_OUTPUTS", "./storage/outputs"),
		StorageTemp:    getEnv("STORAGE_TEMP", "./storage/temp"),
		Environment:    getEnv("ENVIRONMENT", "development"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
