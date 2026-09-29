package main

import (
	"encoding/json"
	"log"
	"time"

	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/config"
	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/database"
	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/models"
	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/rabbitmq"
	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/storage"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	videosProcessedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_videos_processed_total",
			Help: "Total de vídeos processados pelo worker",
		},
		[]string{"status"},
	)
	processingDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "worker_processing_duration_seconds",
			Help:    "Tempo de duração do processamento do vídeo pelo FFmpeg em segundos",
			Buckets: prometheus.DefBuckets,
		},
	)
)

func init() {
	prometheus.MustRegister(videosProcessedTotal)
	prometheus.MustRegister(processingDuration)
}

func main() {
	log.Println("⚙️ Iniciando FIAP X Worker Microservice...")

	cfg := config.LoadConfig()

	if err := storage.EnsureStorageDirs(cfg); err != nil {
		log.Fatalf("Erro nos diretórios de armazenamento do Worker: %v", err)
	}

	db, err := database.InitDB(cfg)
	if err != nil {
		log.Fatalf("Erro ao conectar no banco de dados: %v", err)
	}

	rabbit, err := rabbitmq.NewRabbitClient(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("Erro ao conectar no RabbitMQ: %v", err)
	}
	defer rabbit.Close()

	msgs, err := rabbit.ConsumeProcessQueue()
	if err != nil {
		log.Fatalf("Erro ao iniciar consumo da fila de processamento: %v", err)
	}

	log.Println("⚡ Worker pronto e aguardando tarefas na fila 'video.process.queue'...")

	forever := make(chan struct{})

	go func() {
		for d := range msgs {
			var payload models.ProcessVideoTaskPayload
			if err := json.Unmarshal(d.Body, &payload); err != nil {
				log.Printf("❌ Mensagem corrompida recebida: %v", err)
				d.Nack(false, false) // discard invalid message
				continue
			}

			log.Printf("🎬 Iniciando processamento do Vídeo ID #%d (User ID #%d)...", payload.VideoID, payload.UserID)
			start := time.Now()

			// Update status to PROCESSING
			var video models.Video
			if err := db.First(&video, payload.VideoID).Error; err != nil {
				log.Printf("❌ Vídeo ID #%d não encontrado no banco de dados", payload.VideoID)
				d.Ack(false)
				continue
			}

			video.Status = models.StatusProcessing
			db.Save(&video)

			// Process Video via FFmpeg
			timestamp := time.Now().Format("20060102_150405_999")
			zipFilename, frameCount, err := storage.ProcessVideoToZip(payload.Path, timestamp, cfg)

			duration := time.Since(start).Seconds()
			processingDuration.Observe(duration)

			if err != nil {
				log.Printf("❌ Erro ao processar Vídeo ID #%d: %v", payload.VideoID, err)
				video.Status = models.StatusFailed
				video.ErrorMessage = err.Error()
				db.Save(&video)

				videosProcessedTotal.WithLabelValues("failed").Inc()

				// Publish error payload to error queue for notification service
				errPayload := models.ProcessErrorPayload{
					VideoID:   payload.VideoID,
					UserID:    payload.UserID,
					ErrorMsg:  err.Error(),
					Timestamp: time.Now().Format(time.RFC3339),
				}
				_ = rabbit.PublishErrorNotification(errPayload)

				d.Ack(false)
				continue
			}

			// Success
			now := time.Now()
			video.Status = models.StatusCompleted
			video.ZipPath = zipFilename
			video.FrameCount = frameCount
			video.ErrorMessage = ""
			video.ProcessedAt = &now
			db.Save(&video)

			videosProcessedTotal.WithLabelValues("success").Inc()

			log.Printf("✅ Vídeo ID #%d processado com sucesso! %d frames extraídos em %.2fs. ZIP: %s",
				payload.VideoID, frameCount, duration, zipFilename)

			d.Ack(false)
		}
	}()

	<-forever
}
