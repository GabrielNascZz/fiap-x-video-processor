package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/smtp"

	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/config"
	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/database"
	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/models"
	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/rabbitmq"
)

func main() {
	log.Println("🔔 Iniciando FIAP X Notification Microservice...")

	cfg := config.LoadConfig()

	db, err := database.InitDB(cfg)
	if err != nil {
		log.Fatalf("Erro ao conectar no banco de dados: %v", err)
	}

	rabbit, err := rabbitmq.NewRabbitClient(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("Erro ao conectar no RabbitMQ: %v", err)
	}
	defer rabbit.Close()

	msgs, err := rabbit.ConsumeErrorQueue()
	if err != nil {
		log.Fatalf("Erro ao consumir fila de erros: %v", err)
	}

	log.Println("📩 Notification Service ativo e pronto na fila 'video.error.queue'...")

	forever := make(chan struct{})

	go func() {
		for d := range msgs {
			var errPayload models.ProcessErrorPayload
			if err := json.Unmarshal(d.Body, &errPayload); err != nil {
				log.Printf("❌ Falha ao decodificar notificação de erro: %v", err)
				d.Nack(false, false)
				continue
			}

			log.Printf("⚠️ ALERTA DE ERRO RECEBIDO | Usuário ID #%d | Vídeo ID #%d | Motivo: %s",
				errPayload.UserID, errPayload.VideoID, errPayload.ErrorMsg)

			// Find user email from database
			var user models.User
			userEmail := fmt.Sprintf("user_%d@fiapx.com", errPayload.UserID)
			if err := db.First(&user, errPayload.UserID).Error; err == nil && user.Email != "" {
				userEmail = user.Email
			}

			// Store notification in Database
			notif := models.Notification{
				UserID:  errPayload.UserID,
				VideoID: errPayload.VideoID,
				Type:    "PROCESSING_ERROR",
				Message: fmt.Sprintf("Falha ao processar vídeo #%d: %s", errPayload.VideoID, errPayload.ErrorMsg),
				Status:  "SENT",
			}

			if err := db.Create(&notif).Error; err != nil {
				log.Printf("❌ Erro ao salvar notificação no banco: %v", err)
			}

			// Send real email or simulate email dispatch
			if cfg.SMTPHost != "" {
				err := sendSMTPEmail(cfg, userEmail, "FIAP X - Falha no Processamento do seu Vídeo", notif.Message)
				if err != nil {
					log.Printf("❌ Erro ao enviar e-mail via SMTP (%s): %v", cfg.SMTPHost, err)
				} else {
					log.Printf("📧 E-mail SMTP REAL enviado com sucesso para %s!", userEmail)
				}
			} else {
				log.Printf("📧 [NOTIFICAÇÃO DISPARADA] E-mail enviado para %s | Mensagem: %s", userEmail, notif.Message)
			}

			d.Ack(false)
		}
	}()

	<-forever
}

func sendSMTPEmail(cfg *config.Config, toEmail, subject, body string) error {
	var auth smtp.Auth
	if cfg.SMTPUser != "" && cfg.SMTPPassword != "" && cfg.SMTPHost != "mailhog" {
		auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
	}
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		cfg.SMTPFrom, toEmail, subject, body))
	addr := fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort)
	return smtp.SendMail(addr, auth, cfg.SMTPFrom, []string{toEmail}, msg)
}
