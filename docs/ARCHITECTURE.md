# 🏗️ FIAP X Video Processor - Documentação de Arquitetura

Documentação oficial da arquitetura proposta para a pós-graduação em Software Architecture da FIAP (Fase 5 - Hackathon).

---

## 1. Visão Geral da Solução

O **FIAP X Video Processor** foi desenhado como um **sistema distribuído de microsserviços orientados a eventos (EDA - Event-Driven Architecture)** para processamento de vídeo assíncrono, escalável e resiliente.

### Principais Pilares da Arquitetura:
- **Desacoplamento:** A API HTTP Gateway recebe as requisições de upload de vídeo instantaneamente e as enfileira sem bloquear o usuário.
- **Escalabilidade Horizontal:** Workers de processamento FFmpeg consomem a fila de forma assíncrona e podem ser escalados horizontalmente via Kubernetes HPA.
- **Resiliência a Picos de Tráfego:** Uso de filas duráveis no **RabbitMQ** com mensagens persistentes.
- **Isolamento e Segurança:** Autenticação via **JWT** e senhas hash com **bcrypt**.
- **Notificação de Erros:** Serviço especialista que reage a eventos de erro e registra alertas para os usuários.
- **Observabilidade:** Métricas exportadas via **Prometheus** e visualizadas em dashboards no **Grafana**.

---

## 2. Diagramas de Arquitetura (Mermaid)

### 2.1 Diagrama de Componentes (Visão de Visão de Containers)

```mermaid
graph TB
    Client[📱 Cliente / Web Browser]
    
    subgraph Edge Layer
        API[🚀 API Gateway / HTTP Service<br/>Go + Gin]
    end

    subgraph Messaging & Cache
        Rabbit[🐰 RabbitMQ Topic Exchange<br/>'video.exchange']
        Redis[⚡ Redis Cache / Session]
    end

    subgraph Database
        DB[(🐘 PostgreSQL Database)]
    end

    subgraph Worker Layer
        Worker1[⚙️ Worker 1 - FFmpeg]
        Worker2[⚙️ Worker 2 - FFmpeg]
    end

    subgraph Notification Layer
        Notif[🔔 Notification Service]
    end

    Client -->|HTTP / REST + JWT| API
    API -->|Persiste Upload| DB
    API -->|Publica 'video.process'| Rabbit
    
    Rabbit -->|Consome 'video.process.queue'| Worker1
    Rabbit -->|Consome 'video.process.queue'| Worker2
    
    Worker1 -->|Lê/Grava Status| DB
    Worker2 -->|Lê/Grava Status| DB
    Worker1 -->|Emite Erro 'video.error'| Rabbit
    Worker2 -->|Emite Erro 'video.error'| Rabbit

    Rabbit -->|Consome 'video.error.queue'| Notif
    Notif -->|Salva Notificação| DB
```

---

### 2.2 Fluxo Sequencial de Upload e Processamento (Sequence Diagram)

```mermaid
sequenceDiagram
    autonumber
    actor User as Usuário
    participant API as API Gateway (HTTP)
    participant DB as PostgreSQL
    participant MQ as RabbitMQ
    participant Worker as Worker Microservice
    participant Notif as Notification Service

    User->>API: POST /api/v1/auth/login
    API-->>User: Retorna JWT Token

    User->>API: POST /api/v1/videos/upload (com JWT)
    API->>DB: Salva registro do Vídeo (Status: PENDING)
    API->>MQ: Publica payload na fila 'video.process'
    API-->>User: 202 Accepted (ID do Vídeo, Status: PENDING)

    MQ->>Worker: Consome evento 'video.process'
    Worker->>DB: Atualiza status do Vídeo para PROCESSING
    
    alt Processamento com Sucesso
        Worker->>Worker: Executa FFmpeg (Extrai 1 fps PNG)
        Worker->>Worker: Compacta PNGs em .ZIP
        Worker->>DB: Atualiza status para COMPLETED (caminho zip e frame_count)
    else Falha no Processamento
        Worker->>DB: Atualiza status para FAILED (com error_message)
        Worker->>MQ: Publica mensagem na fila 'video.error'
        MQ->>Notif: Consome mensagem 'video.error'
        Notif->>DB: Salva registro na tabela 'notifications'
    end

    User->>API: GET /api/v1/videos
    API->>DB: Busca vídeos do usuário
    API-->>User: Retorna lista com status atualizado

    User->>API: GET /api/v1/videos/:id/download
    API-->>User: Retorna arquivo .ZIP com os frames
```

---

## 3. Modelo de Dados (ERD)

```mermaid
erDiagram
    USERS ||--o{ VIDEOS : "possui"
    USERS ||--o{ NOTIFICATIONS : "recebe"
    VIDEOS ||--o{ NOTIFICATIONS : "gera"

    USERS {
        uint id PK
        string username UK
        string email UK
        string password_hash
        datetime created_at
        datetime updated_at
    }

    VIDEOS {
        uint id PK
        uint user_id FK
        string original_name
        string file_path
        string status
        string zip_path
        int frame_count
        string error_message
        datetime created_at
        datetime updated_at
        datetime processed_at
    }

    NOTIFICATIONS {
        uint id PK
        uint user_id FK
        uint video_id FK
        string type
        string message
        string status
        datetime created_at
    }
```

---

## 4. Padrões de Qualidade e Escalabilidade

1. **Horizontal Pod Autoscaling (HPA):** O deployment dos workers no Kubernetes escala automaticamente com base no uso de CPU (alvo: 70%).
2. **Dead Letter Queue (DLQ):** Erros de processamento não travam a fila principal e são redirecionados para rastreamento.
3. **Persistência Volátil Separada de Armazenamento:** Arquivos são armazenados em volumes compartilhados (`shared_storage`) permitindo resiliência em falhas de pods.
