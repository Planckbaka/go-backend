package model

import (
	"github.com/pgvector/pgvector-go"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type File struct {
	gorm.Model
	ContentType      string          `gorm:"type:varchar(20);not null"`
	FileName         string          `gorm:"type:varchar(255);not null"`
	OriginalFilePath string          `gorm:"type:text;not null"`
	FilePath         string          `gorm:"type:text"`
	Size             int64           `gorm:"type:bigint;not null"`
	Metadata         datatypes.JSON  `gorm:"type:jsonb;default:'{}'"`
	Caption          string          `gorm:"type:text"`
	Tag              datatypes.JSON  `gorm:"type:jsonb;default:'{}'"`
	Vector           pgvector.Vector `gorm:"type:vector(512)"`
}
