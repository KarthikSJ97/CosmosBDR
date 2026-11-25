package schema

import (
	"time"
)

type Chunk struct {
	ID                  string    `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	ChunkHash           string    `gorm:"type:varchar(64);uniqueIndex;not null"`
	SeaweedKey          string    `gorm:"type:text;not null"`
	SizeBytes           int64     `gorm:"not null"`
	CompressedSizeBytes int64     `gorm:"not null"`
	CompressionAlg      string    `gorm:"type:varchar(16);not null;default:none"`
	Encrypted           bool      `gorm:"not null;default:false"`
	SeaweedVersionID    string    `gorm:"type:text"`
	RetainUntil         *time.Time
	ReferenceCount      int       `gorm:"default:1"`
	CreatedAt           time.Time `gorm:"autoCreateTime"`

	// Relations
	ItemChunks []ItemChunk `gorm:"foreignKey:ChunkID;constraint:OnDelete:RESTRICT"`
}
