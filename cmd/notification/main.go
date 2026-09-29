package main

import (
	"encoding/json"
	"fmt"
	"log"

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
			} else {
				log.Printf("📧 [SIMULAÇÃO DE EMAIL/ALERT] Notificação enviada para Usuário ID #%d!", errPayload.UserID)
			}

			d.Ack(false)
		}
	}()

	<-forever
}
