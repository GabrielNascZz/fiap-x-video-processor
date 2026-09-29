package models

import (
	"time"

	"gorm.io/gorm"
)

type VideoStatus string

const (
	StatusPending    VideoStatus = "PENDING"
	StatusProcessing VideoStatus = "PROCESSING"
	StatusCompleted  VideoStatus = "COMPLETED"
	StatusFailed     VideoStatus = "FAILED"
)

type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Username     string         `gorm:"uniqueIndex;not null" json:"username"`
	Email        string         `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string         `gorm:"not null" json:"-"`
	Videos       []Video        `gorm:"foreignKey:UserID" json:"videos,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

type Video struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	UserID       uint           `gorm:"not null;index" json:"user_id"`
	User         User           `gorm:"foreignKey:UserID" json:"-"`
	OriginalName string         `gorm:"not null" json:"original_name"`
	FilePath     string         `gorm:"not null" json:"file_path"`
	Status       VideoStatus    `gorm:"type:varchar(20);default:'PENDING';index" json:"status"`
	ZipPath      string         `json:"zip_path,omitempty"`
	FrameCount   int            `json:"frame_count,omitempty"`
	ErrorMessage string         `json:"error_message,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	ProcessedAt  *time.Time     `json:"processed_at,omitempty"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

type Notification struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	VideoID   uint      `gorm:"not null;index" json:"video_id"`
	Type      string    `gorm:"type:varchar(50)" json:"type"`
	Message   string    `gorm:"type:text" json:"message"`
	Status    string    `gorm:"type:varchar(20);default:'SENT'" json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type ProcessVideoTaskPayload struct {
	VideoID uint   `json:"video_id"`
	UserID  uint   `json:"user_id"`
	Path    string `json:"path"`
}

type ProcessErrorPayload struct {
	VideoID   uint   `json:"video_id"`
	UserID    uint   `json:"user_id"`
	ErrorMsg  string `json:"error_msg"`
	Timestamp string `json:"timestamp"`
}
