# 📡 FIAP X Video Processor - Documentação de API REST

Especificação completa das rotas HTTP disponíveis na API Gateway (`http://localhost:8080`).

---

## 1. Autenticação & Usuários

### `POST /api/v1/auth/register`
Cadastra um novo usuário no sistema.

**Body (JSON):**
```json
{
  "username": "usuario1",
  "email": "usuario1@fiapx.com",
  "password": "senhaSegura123"
}
```

**Resposta de Sucesso (201 Created):**
```json
{
  "message": "Usuário cadastrado com sucesso",
  "user_id": 1
}
```

---

### `POST /api/v1/auth/login`
Autentica o usuário e retorna o Token JWT.

**Body (JSON):**
```json
{
  "username": "usuario1",
  "password": "senhaSegura123"
}
```

**Resposta de Sucesso (200 OK):**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user_id": 1,
  "username": "usuario1"
}
```

---

## 2. Processamento de Vídeos (Rotas Protegidas)

*Nota: Todas as rotas abaixo requerem o cabeçalho HTTP: `Authorization: Bearer <SEU_TOKEN_JWT>`.*

---

### `POST /api/v1/videos/upload`
Envia um arquivo de vídeo para processamento assíncrono.

**Headers:**
- `Authorization: Bearer <TOKEN>`
- `Content-Type: multipart/form-data`

**Body (Form Data):**
- `video`: arquivo de vídeo (`.mp4`, `.avi`, `.mov`, `.mkv`)

**Resposta de Sucesso (202 Accepted):**
```json
{
  "message": "Vídeo recebido e adicionado à fila de processamento assíncrono!",
  "status": "PENDING",
  "video_id": 1
}
```

---

### `GET /api/v1/videos`
Lista os vídeos pertencentes ao usuário autenticado e seus respectivos status.

**Headers:**
- `Authorization: Bearer <TOKEN>`

**Resposta de Sucesso (200 OK):**
```json
{
  "total": 1,
  "videos": [
    {
      "id": 1,
      "user_id": 1,
      "original_name": "meuvideo.mp4",
      "file_path": "/app/storage/uploads/1_20260929_meuvideo.mp4",
      "status": "COMPLETED",
      "zip_path": "frames_20260929_114500.zip",
      "frame_count": 45,
      "error_message": "",
      "created_at": "2026-09-29T11:45:00Z",
      "updated_at": "2026-09-29T11:45:10Z",
      "processed_at": "2026-09-29T11:45:10Z"
    }
  ]
}
```

---

### `GET /api/v1/videos/:id/download`
Faz o download do arquivo `.ZIP` contendo as imagens dos frames extraídos.

**Headers:**
- `Authorization: Bearer <TOKEN>`

**Resposta de Sucesso (200 OK):**
- Retorna o arquivo binário `.ZIP`.

---

## 3. Observabilidade e Saúde

- `GET /health` -> Retorna o status de saúde do serviço HTTP.
- `GET /metrics` -> Retorna as métricas em formato Prometheus.
