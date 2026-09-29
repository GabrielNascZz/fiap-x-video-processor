package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/auth"
	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/config"
	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/database"
	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/models"
	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/rabbitmq"
	"github.com/GabrielNascZz/fiap-x-video-processor/pkg/storage"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total de requisições HTTP recebidas",
		},
		[]string{"method", "endpoint", "status"},
	)
	videosUploadedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "videos_uploaded_total",
			Help: "Total de vídeos enviados para processamento",
		},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(videosUploadedTotal)
}

func main() {
	cfg := config.LoadConfig()

	if err := storage.EnsureStorageDirs(cfg); err != nil {
		log.Fatalf("Erro na inicialização dos diretórios de armazenamento: %v", err)
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

	r := gin.Default()

	// Metrics middleware
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		statusStr := strconv.Itoa(c.Writer.Status())
		httpRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), statusStr).Inc()
		_ = start
	})

	// CORS Middleware
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	r.Static("/storage/outputs", cfg.StorageOutputs)

	// Public Routes
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "UP", "timestamp": time.Now().Format(time.RFC3339)})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.GET("/", func(c *gin.Context) {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(200, getFrontendHTML())
	})

	api := r.Group("/api/v1")
	{
		// Auth routes
		api.POST("/auth/register", handleRegister(db))
		api.POST("/auth/login", handleLogin(db, cfg.JWTSecret))

		// Protected routes
		protected := api.Group("")
		protected.Use(auth.AuthMiddleware(cfg.JWTSecret))
		{
			protected.POST("/videos/upload", handleUploadVideo(db, rabbit, cfg))
			protected.GET("/videos", handleListVideos(db))
			protected.GET("/videos/:id", handleGetVideo(db))
			protected.GET("/videos/:id/download", handleDownloadVideoZip(db, cfg))
			protected.GET("/notifications", handleListNotifications(db))
		}
	}

	fmt.Printf("🚀 FIAP X API Gateway iniciado na porta %s\n", cfg.Port)
	log.Fatal(r.Run(":" + cfg.Port))
}

type AuthRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email"`
	Password string `json:"password" binding:"required"`
}

func handleRegister(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req AuthRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Parâmetros inválidos: " + err.Error()})
			return
		}

		if req.Email == "" {
			req.Email = req.Username + "@fiapx.com"
		}

		hashedPassword, err := auth.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao processar senha"})
			return
		}

		user := models.User{
			Username:     req.Username,
			Email:        req.Email,
			PasswordHash: hashedPassword,
		}

		if err := db.Create(&user).Error; err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Usuário ou e-mail já cadastrado"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "Usuário cadastrado com sucesso",
			"user_id": user.ID,
		})
	}
}

func handleLogin(db *gorm.DB, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req AuthRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Parâmetros inválidos"})
			return
		}

		var user models.User
		if err := db.Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Credenciais inválidas"})
			return
		}

		if !auth.CheckPasswordHash(req.Password, user.PasswordHash) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Credenciais inválidas"})
			return
		}

		token, err := auth.GenerateToken(user.ID, user.Username, jwtSecret)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao gerar token JWT"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token":    token,
			"username": user.Username,
			"user_id":  user.ID,
		})
	}
}

func handleUploadVideo(db *gorm.DB, rabbit *rabbitmq.RabbitClient, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.MustGet("user_id").(uint)

		file, header, err := c.Request.FormFile("video")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Envio de arquivo obrigatório ('video')"})
			return
		}
		defer file.Close()

		if !storage.IsValidVideoFile(header.Filename) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Formato de vídeo não suportado. Suportados: mp4, avi, mov, mkv, wmv, flv, webm"})
			return
		}

		timestamp := time.Now().Format("20060102_150405_999")
		safeFilename := fmt.Sprintf("%d_%s_%s", userID, timestamp, header.Filename)
		destPath := filepath.Join(cfg.StorageUploads, safeFilename)

		if err := c.SaveUploadedFile(header, destPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao salvar arquivo no servidor: " + err.Error()})
			return
		}

		videoRecord := models.Video{
			UserID:       userID,
			OriginalName: header.Filename,
			FilePath:     destPath,
			Status:       models.StatusPending,
		}

		if err := db.Create(&videoRecord).Error; err != nil {
			os.Remove(destPath)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao registrar vídeo no banco de dados"})
			return
		}

		// Publish message to RabbitMQ for asynchronous processing
		payload := models.ProcessVideoTaskPayload{
			VideoID: videoRecord.ID,
			UserID:  userID,
			Path:    destPath,
		}

		if err := rabbit.PublishProcessTask(payload); err != nil {
			videoRecord.Status = models.StatusFailed
			videoRecord.ErrorMessage = "Erro ao enfileirar mensagem no RabbitMQ"
			db.Save(&videoRecord)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao enviar tarefa para fila de mensageria"})
			return
		}

		videosUploadedTotal.Inc()

		c.JSON(http.StatusAccepted, gin.H{
			"message":  "Vídeo recebido e adicionado à fila de processamento assíncrono!",
			"video_id": videoRecord.ID,
			"status":   models.StatusPending,
		})
	}
}

func handleListVideos(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.MustGet("user_id").(uint)

		var videos []models.Video
		if err := db.Where("user_id = ?", userID).Order("id desc").Find(&videos).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar vídeos"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"total":  len(videos),
			"videos": videos,
		})
	}
}

func handleGetVideo(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.MustGet("user_id").(uint)
		videoID := c.Param("id")

		var video models.Video
		if err := db.Where("id = ? AND user_id = ?", videoID, userID).First(&video).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Vídeo não encontrado"})
			return
		}

		c.JSON(http.StatusOK, video)
	}
}

func handleDownloadVideoZip(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.MustGet("user_id").(uint)
		videoID := c.Param("id")

		var video models.Video
		if err := db.Where("id = ? AND user_id = ?", videoID, userID).First(&video).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Vídeo não encontrado"})
			return
		}

		if video.Status != models.StatusCompleted || video.ZipPath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Processamento do vídeo ainda não foi concluído ou falhou"})
			return
		}

		fullZipPath := filepath.Join(cfg.StorageOutputs, video.ZipPath)
		if _, err := os.Stat(fullZipPath); os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Arquivo ZIP não encontrado no armazenamento"})
			return
		}

		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", video.ZipPath))
		c.Header("Content-Type", "application/zip")
		c.File(fullZipPath)
	}
}

func handleListNotifications(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.MustGet("user_id").(uint)

		var notifications []models.Notification
		if err := db.Where("user_id = ?", userID).Order("id desc").Find(&notifications).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Erro ao buscar notificações"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"total":         len(notifications),
			"notifications": notifications,
		})
	}
}

func getFrontendHTML() string {
	return `<!DOCTYPE html>
<html lang="pt-BR">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>FIAP X - Plataforma de Processamento de Vídeos</title>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;600;700&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-color: #0f172a;
            --card-bg: #1e293b;
            --accent: #e11d48;
            --accent-hover: #be123c;
            --text-main: #f8fafc;
            --text-muted: #94a3b8;
            --border: #334155;
            --success: #10b981;
            --warning: #f59e0b;
            --danger: #ef4444;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; font-family: 'Inter', sans-serif; }
        body { background-color: var(--bg-color); color: var(--text-main); padding: 40px 20px; display: flex; justify-content: center; }
        .container { max-width: 900px; width: 100%; }
        .header { text-align: center; margin-bottom: 40px; }
        .header h1 { font-size: 2.5rem; color: var(--text-main); margin-bottom: 10px; }
        .header h1 span { color: var(--accent); }
        .header p { color: var(--text-muted); font-size: 1.1rem; }
        .card { background: var(--card-bg); border: 1px solid var(--border); border-radius: 12px; padding: 28px; margin-bottom: 24px; box-shadow: 0 10px 25px -5px rgba(0,0,0,0.3); }
        .card h2 { margin-bottom: 20px; font-size: 1.4rem; border-bottom: 1px solid var(--border); padding-bottom: 10px; }
        .form-group { margin-bottom: 16px; }
        label { display: block; margin-bottom: 8px; color: var(--text-muted); font-size: 0.9rem; }
        input[type="text"], input[type="password"], input[type="file"] { width: 100%; padding: 12px; border-radius: 8px; background: #0f172a; border: 1px solid var(--border); color: var(--text-main); outline: none; }
        input[type="text"]:focus, input[type="password"]:focus { border-color: var(--accent); }
        .btn { background: var(--accent); color: white; border: none; padding: 12px 24px; border-radius: 8px; font-weight: 600; cursor: pointer; transition: 0.2s; font-size: 1rem; width: 100%; }
        .btn:hover { background: var(--accent-hover); }
        .btn-success { background: var(--success); }
        .btn-success:hover { background: #059669; }
        .grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 20px; }
        .video-item { background: #0f172a; border: 1px solid var(--border); border-radius: 8px; padding: 16px; margin-bottom: 12px; display: flex; justify-content: space-between; align-items: center; }
        .badge { padding: 4px 10px; border-radius: 20px; font-size: 0.8rem; font-weight: 600; text-transform: uppercase; }
        .badge-PENDING { background: rgba(245, 158, 11, 0.2); color: var(--warning); border: 1px solid var(--warning); }
        .badge-PROCESSING { background: rgba(59, 130, 246, 0.2); color: #3b82f6; border: 1px solid #3b82f6; }
        .badge-COMPLETED { background: rgba(16, 185, 129, 0.2); color: var(--success); border: 1px solid var(--success); }
        .badge-FAILED { background: rgba(239, 68, 68, 0.2); color: var(--danger); border: 1px solid var(--danger); }
        .auth-bar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; padding: 12px 20px; background: #1e293b; border-radius: 8px; border: 1px solid var(--border); }
        .auth-bar span { color: var(--success); font-weight: 600; }
        .hidden { display: none; }
        .alert { padding: 12px; border-radius: 8px; margin-bottom: 16px; font-size: 0.9rem; }
        .alert-error { background: rgba(239,68,68,0.2); color: #fca5a5; border: 1px solid var(--danger); }
        .alert-info { background: rgba(59,130,246,0.2); color: #93c5fd; border: 1px solid #3b82f6; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🎬 <span>FIAP X</span> Video Processor</h1>
            <p>Plataforma Assíncrona de Extração de Frames e Compactação</p>
        </div>

        <div id="authSection" class="grid-2">
            <div class="card">
                <h2>🔑 Login</h2>
                <div id="loginAlert"></div>
                <form id="loginForm">
                    <div class="form-group">
                        <label>Usuário / Email</label>
                        <input type="text" id="loginUsername" value="admin" required>
                    </div>
                    <div class="form-group">
                        <label>Senha</label>
                        <input type="password" id="loginPassword" value="fiap123" required>
                    </div>
                    <button type="submit" class="btn">Entrar</button>
                </form>
            </div>

            <div class="card">
                <h2>📝 Novo Cadastro</h2>
                <div id="registerAlert"></div>
                <form id="registerForm">
                    <div class="form-group">
                        <label>Nome de Usuário</label>
                        <input type="text" id="regUsername" required>
                    </div>
                    <div class="form-group">
                        <label>Email</label>
                        <input type="text" id="regEmail" required>
                    </div>
                    <div class="form-group">
                        <label>Senha</label>
                        <input type="password" id="regPassword" required>
                    </div>
                    <button type="submit" class="btn btn-success">Cadastrar</button>
                </form>
            </div>
        </div>

        <div id="appSection" class="hidden">
            <div class="auth-bar">
                <div>Conectado como: <span id="currentUser"></span></div>
                <button onclick="logout()" class="btn" style="width: auto; padding: 6px 16px;">Sair</button>
            </div>

            <div class="card">
                <h2>📤 Enviar Vídeo para Processamento Assíncrono</h2>
                <div id="uploadAlert"></div>
                <form id="uploadForm">
                    <div class="form-group">
                        <label>Selecione um arquivo de vídeo (MP4, AVI, MOV, MKV):</label>
                        <input type="file" id="videoFile" accept="video/*" required>
                    </div>
                    <button type="submit" class="btn" id="uploadBtn">🚀 Enviar para Fila de Processamento</button>
                </form>
            </div>

            <div class="card">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 15px;">
                    <h2>📋 Meus Vídeos Processados</h2>
                    <button onclick="loadVideos()" class="btn" style="width: auto; padding: 6px 12px; font-size: 0.85rem;">🔄 Atualizar Status</button>
                </div>
                <div id="videosList">Carregando...</div>
            </div>

            <div class="card">
                <h2>🔔 Notificações e Alertas de Erros</h2>
                <div id="notificationsList">Nenhuma notificação por enquanto.</div>
            </div>
        </div>
    </div>

    <script>
        let token = localStorage.getItem('jwt_token');
        let username = localStorage.getItem('username');

        if (token) showApp();

        document.getElementById('loginForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            try {
                const res = await fetch('/api/v1/auth/login', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({
                        username: document.getElementById('loginUsername').value,
                        password: document.getElementById('loginPassword').value
                    })
                });
                const data = await res.json();
                if (res.ok) {
                    token = data.token;
                    username = data.username;
                    localStorage.setItem('jwt_token', token);
                    localStorage.setItem('username', username);
                    showApp();
                } else {
                    document.getElementById('loginAlert').innerHTML = `<div class="alert alert-error">${data.error}</div>`;
                }
            } catch (err) {
                document.getElementById('loginAlert').innerHTML = `<div class="alert alert-error">Erro de conexão com o servidor</div>`;
            }
        });

        document.getElementById('registerForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            try {
                const res = await fetch('/api/v1/auth/register', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({
                        username: document.getElementById('regUsername').value,
                        email: document.getElementById('regEmail').value,
                        password: document.getElementById('regPassword').value
                    })
                });
                const data = await res.json();
                if (res.ok) {
                    document.getElementById('registerAlert').innerHTML = `<div class="alert alert-info">Cadastro realizado! Faça login.</div>`;
                } else {
                    document.getElementById('registerAlert').innerHTML = `<div class="alert alert-error">${data.error}</div>`;
                }
            } catch (err) {
                document.getElementById('registerAlert').innerHTML = `<div class="alert alert-error">Erro ao cadastrar</div>`;
            }
        });

        document.getElementById('uploadForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            const fileInput = document.getElementById('videoFile');
            if (!fileInput.files[0]) return;

            const formData = new FormData();
            formData.append('video', fileInput.files[0]);

            const uploadBtn = document.getElementById('uploadBtn');
            uploadBtn.disabled = true;
            uploadBtn.innerText = '⏳ Enviando...';

            try {
                const res = await fetch('/api/v1/videos/upload', {
                    method: 'POST',
                    headers: {'Authorization': `Bearer ${token}`},
                    body: formData
                });
                const data = await res.json();
                if (res.ok) {
                    document.getElementById('uploadAlert').innerHTML = `<div class="alert alert-info">${data.message}</div>`;
                    fileInput.value = '';
                    loadVideos();
                } else {
                    document.getElementById('uploadAlert').innerHTML = `<div class="alert alert-error">${data.error}</div>`;
                }
            } catch (err) {
                document.getElementById('uploadAlert').innerHTML = `<div class="alert alert-error">Erro no upload</div>`;
            } finally {
                uploadBtn.disabled = false;
                uploadBtn.innerText = '🚀 Enviar para Fila de Processamento';
            }
        });

        function showApp() {
            document.getElementById('authSection').classList.add('hidden');
            document.getElementById('appSection').classList.remove('hidden');
            document.getElementById('currentUser').innerText = username;
            loadVideos();
            loadNotifications();
            setInterval(loadVideos, 5000);
        }

        function logout() {
            localStorage.clear();
            location.reload();
        }

        async function loadVideos() {
            if (!token) return;
            try {
                const res = await fetch('/api/v1/videos', {
                    headers: {'Authorization': `Bearer ${token}`}
                });
                const data = await res.json();
                const container = document.getElementById('videosList');
                if (data.videos && data.videos.length > 0) {
                    container.innerHTML = data.videos.map(v => `
                        <div class="video-item">
                            <div>
                                <strong>${v.original_name}</strong>
                                <div style="font-size: 0.8rem; color: var(--text-muted); margin-top: 4px;">
                                    Criado em: ${new Date(v.created_at).toLocaleString('pt-BR')}
                                    ${v.frame_count ? ` | 📸 ${v.frame_count} frames` : ''}
                                    ${v.error_message ? ` | ⚠️ ${v.error_message}` : ''}
                                </div>
                            </div>
                            <div style="display: flex; align-items: center; gap: 12px;">
                                <span class="badge badge-${v.status}">${v.status}</span>
                                ${v.status === 'COMPLETED' ? `
                                    <a href="/api/v1/videos/${v.id}/download" 
                                       headers="Authorization: Bearer ${token}" 
                                       onclick="downloadZip(event, ${v.id}, '${v.zip_path}')"
                                       class="btn btn-success" style="padding: 6px 12px; font-size: 0.85rem; text-decoration: none;">⬇️ Download ZIP</a>
                                ` : ''}
                            </div>
                        </div>
                    `).join('');
                } else {
                    container.innerHTML = `<p style="color: var(--text-muted);">Nenhum vídeo enviado ainda.</p>`;
                }
            } catch (err) {}
        }

        async function downloadZip(event, videoId, filename) {
            event.preventDefault();
            const res = await fetch(`/api/v1/videos/${videoId}/download`, {
                headers: {'Authorization': `Bearer ${token}`}
            });
            if (res.ok) {
                const blob = await res.blob();
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = filename || `frames_${videoId}.zip`;
                document.body.appendChild(a);
                a.click();
                a.remove();
            } else {
                alert('Erro ao baixar arquivo ZIP');
            }
        }

        async function loadNotifications() {
            if (!token) return;
            try {
                const res = await fetch('/api/v1/notifications', {
                    headers: {'Authorization': `Bearer ${token}`}
                });
                const data = await res.json();
                const container = document.getElementById('notificationsList');
                if (data.notifications && data.notifications.length > 0) {
                    container.innerHTML = data.notifications.map(n => `
                        <div class="alert alert-error" style="margin-bottom: 8px;">
                            <strong>[${n.type}]</strong> ${n.message} <em>(${new Date(n.created_at).toLocaleString('pt-BR')})</em>
                        </div>
                    `).join('');
                } else {
                    container.innerHTML = `<p style="color: var(--text-muted);">Nenhum alerta de erro registrado.</p>`;
                }
            } catch (err) {}
        }
    </script>
</body>
</html>`
}
