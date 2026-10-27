package schema

import (
	"time"
)

type JobItem struct {
	ID            string    `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	JobID         string    `gorm:"type:uuid;not null;index"`
	ItemPath      string    `gorm:"type:text;not null"`
	OriginalSize  int64     `gorm:"not null"`
	ProcessedSize int64     `gorm:"not null"`
	Checksum      string    `gorm:"type:varchar(64);not null"`
	Status        string    `gorm:"type:varchar(50);not null;default:'PENDING'"`
	ItemMetadata  []byte    `gorm:"type:jsonb;default:'{}'"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`

	// Relations
	Job        Job         `gorm:"foreignKey:JobID"`
	ItemChunks []ItemChunk `gorm:"foreignKey:ItemID;constraint:OnDelete:CASCADE"`
}
