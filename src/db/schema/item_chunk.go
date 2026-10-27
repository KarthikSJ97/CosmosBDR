package schema

type ItemChunk struct {
	ItemID      string `gorm:"type:uuid;primaryKey;autoIncrement:false"`
	ChunkID     string `gorm:"type:uuid;not null;index"`
	ChunkIndex  int    `gorm:"primaryKey;autoIncrement:false"`
	OffsetBytes int64  `gorm:"not null"`
	LengthBytes int    `gorm:"not null"`

	// Relations
	JobItem JobItem `gorm:"foreignKey:ItemID"`
	Chunk   Chunk   `gorm:"foreignKey:ChunkID"`
}
