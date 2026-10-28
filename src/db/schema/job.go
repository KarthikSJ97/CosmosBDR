package schema

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Job struct {
	ID           string         `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	FlowType     string         `gorm:"type:varchar(50);not null;index:idx_jobs_flow_status"`
	Status       string         `gorm:"type:varchar(50);not null;default:'CREATED';index:idx_jobs_flow_status"`
	SourcePath   string         `gorm:"type:text;not null"`
	TargetPath   *string        `gorm:"type:text"`
	TotalItems   int64          `gorm:"default:0"`
	TotalBytes   int64          `gorm:"default:0"`
	ErrorMessage *string        `gorm:"type:text"`
	Payload      datatypes.JSON `gorm:"type:jsonb;default:'{}'"`
	CreatedAt    time.Time      `gorm:"autoCreateTime"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`

	// Relations
	JobItems []JobItem `gorm:"foreignKey:JobID;constraint:OnDelete:CASCADE"`
}
