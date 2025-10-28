package model

import (
	"github.com/pgvector/pgvector-go"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// FileType 文件类型枚举
type FileType string

const (
	FileTypeImage    FileType = "image"
	FileTypeDocument FileType = "document"
)

type File struct {
	gorm.Model
	ContentType      string           `gorm:"type:varchar(20);not null"`
	FileType         FileType         `gorm:"type:varchar(20)"`
	FileName         string           `gorm:"type:varchar(255);not null"`
	OriginalFilePath string           `gorm:"type:text;not null"`
	FilePath         string           `gorm:"type:text"`
	Size             int64            `gorm:"type:bigint;not null"`
	Metadata         datatypes.JSON   `gorm:"type:jsonb;default:'{}'"`
	Caption          string           `gorm:"type:text"`
	Tag              datatypes.JSON   `gorm:"type:jsonb;default:'{}'"`
	Vector           *pgvector.Vector `gorm:"type:vector(512)；"`
	ErrorMessage     string           `gorm:"type:text"`
}

// ImageMetadata 图片元数据结构
type ImageMetadata struct {
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	ColorSpace  string `json:"color_space"`
	Compression string `json:"compression"`
	DPI         int    `json:"dpi"`
	HasAlpha    bool   `json:"has_alpha"`
}

// DocumentMetadata 文档元数据结构
type DocumentMetadata struct {
	PageCount    int      `json:"page_count"`
	WordCount    int      `json:"word_count"`
	Language     string   `json:"language"`
	Author       string   `json:"author"`
	Title        string   `json:"title"`
	Keywords     []string `json:"keywords"`
	CreationDate string   `json:"creation_date"`
}
