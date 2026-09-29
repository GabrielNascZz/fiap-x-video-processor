package database

import (
	"fmt"
	"log"
	"time"

	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/auth"
	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/config"
	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort, cfg.DBSSLMode,
	)

	var db *gorm.DB
	var err error

	// Retry loop for DB connection in Docker startup
	for i := 0; i < 10; i++ {
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		if err == nil {
			break
		}
		log.Printf("Aguardando banco de dados ficar disponível (%d/10)...: %v", i+1, err)
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("falha ao conectar ao banco de dados: %w", err)
	}

	log.Println("✅ Conexão com PostgreSQL estabelecida com sucesso!")

	// Auto-Migrate schema
	err = db.AutoMigrate(&models.User{}, &models.Video{}, &models.Notification{})
	if err != nil {
		return nil, fmt.Errorf("falha ao rodar migrações: %w", err)
	}

	log.Println("✅ Migrações do banco de dados executadas com sucesso!")

	// Seed default user if none exists
	seedDefaultUser(db)

	return db, nil
}

func seedDefaultUser(db *gorm.DB) {
	var count int64
	db.Model(&models.User{}).Count(&count)
	if count == 0 {
		hashedPassword, _ := auth.HashPassword("fiap123")
		defaultUser := models.User{
			Username:     "admin",
			Email:        "admin@fiapx.com",
			PasswordHash: hashedPassword,
		}
		if err := db.Create(&defaultUser).Error; err == nil {
			log.Println("👤 Usuário padrão criado: admin / fiap123")
		}
	}
}
