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
	SMTPHost        string
	SMTPPort        string
	SMTPUser        string
	SMTPPassword    string
	SMTPFrom        string
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
		SMTPHost:       getEnv("SMTP_HOST", getEnv("Smtp__Host", "mailhog")),
		SMTPPort:       getEnv("SMTP_PORT", getEnv("Smtp__Port", "1025")),
		SMTPUser:       getEnv("SMTP_USER", getEnv("Smtp__Email", "notificacoes@techgarage.com")),
		SMTPPassword:   getEnv("SMTP_PASSWORD", getEnv("Smtp__Password", "password123")),
		SMTPFrom:       getEnv("SMTP_FROM", getEnv("Smtp__Email", "notificacoes@techgarage.com")),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
