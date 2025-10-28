package service

import (
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Planckbaka/go-backend/internal/model"

	"gorm.io/gorm"
)

// FileProcessor 文件处理器结构体
// Description: 负责统一处理文件格式转换，支持图片转jpg和文档转md
type FileProcessor struct {
	dbConn       *gorm.DB
	UploadDir    string // original file directory
	ProcessedDir string // processed file directory
}

// NewFileProcessor 创建新的文件处理器实例
// Description: 初始化文件处理器，设置原始文件目录和处理后的文件目录
// Parameters:
//   - db: 后端数据库
//   - uploadDir: 原始文件目录
//   - processedDir: 处理后的文件目录
//
// Returns:
//   - *FileProcessor: 新的文件处理器实例
func NewFileProcessor(db *gorm.DB, uploadDir string, processedDir string) *FileProcessor {
	return &FileProcessor{
		dbConn:       db,
		UploadDir:    uploadDir,
		ProcessedDir: processedDir,
	}
}

// ProcessFile 统一文件处理入口函数
// 说明：根据文件类型自动选择转换策略，返回处理后的文件路径
func (fp *FileProcessor) ProcessUploadedFile(file *multipart.FileHeader, fileID uint) error {
	//确定文件类型

	fileType, err := fp.determineFileType(file.Filename)
	if err != nil {
		return err
	}
	//更新文件类型：图片、文档
	fp.dbConn.Model(&model.File{}).Where("id = ?", fileID).Updates(map[string]interface{}{
		"file_type":  fileType,
		"updated_at": time.Now(),
	})

	//异步处理函数
	go fp.processFileAsync(fileID)
	return nil
}

// processFileAsync 异步处理文件
func (fp *FileProcessor) processFileAsync(fileID uint) {
	fp.dbConn.Model(&model.File{}).Where("id = ?", fileID).Updates(map[string]interface{}{
		"updated_at": time.Now(),
	})

	//根据fileID,获取数据库记录
	var record model.File
	if err := fp.dbConn.First(&record, "id = ?", fileID).Error; err != nil {
		fp.updateProcessError(fileID, fmt.Errorf("查找文件记录失败: %w", err))
		return
	}

	var processedPath string
	var metadata interface{}
	var err error

	//根据文件类型进行处理
	switch record.FileType {
	case model.FileTypeImage:
		processedPath, metadata, err = fp.processImage(&record)
	case model.FileTypeDocument:
		processedPath, metadata, err = fp.processDocument(&record)
	default:
		err = fmt.Errorf("未知文件类型: %s", record.FileType)
	}

	if err != nil {
		fp.updateProcessError(fileID, err)
		return
	}

	// 更新处理结果
	fp.updateProcessSuccess(fileID, processedPath, metadata)
}

// 辅助方法
func (s *FileProcessor) determineFileType(filename string) (model.FileType, error) {
	var ext = strings.ToLower(filepath.Ext(filename))
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif"}
	documentExts := []string{".txt", ".md", ".html", ".htm"}

	for _, imgExt := range imageExts {
		if ext == imgExt {
			return model.FileTypeImage, nil
		}
	}

	for _, docExt := range documentExts {
		if ext == docExt {
			return model.FileTypeDocument, nil
		}
	}

	return "", fmt.Errorf("不支持的文件扩展名: %s", ext)
}

// processImage 处理图片文件
func (fp *FileProcessor) processImage(record *model.File) (string, *model.ImageMetadata, error) {
	var fileNumber = strings.TrimSuffix(record.FileName, filepath.Ext(record.FileName)) // 1. 打开原始图片
	file, err := os.Open(record.OriginalFilePath)
	if err != nil {
		return "", nil, fmt.Errorf("打开图片文件失败: %w", err)
	}
	defer file.Close()

	// 2. 解码图片
	var img image.Image

	switch filepath.Ext(record.FileName) {
	case ".jpg", ".jpeg":
		img, err = jpeg.Decode(file)

	case ".png":
		img, err = png.Decode(file)

	case ".gif":
		img, err = gif.Decode(file)

	default:
		return "", nil, fmt.Errorf("不支持的图片格式: %s", record.ContentType)
	}

	if err != nil {
		return "", nil, fmt.Errorf("解码图片失败: %w", err)
	}

	// 3. 生成处理后的文件路径
	processedPath := filepath.Join(fp.ProcessedDir, fileNumber+".jpg")
	createDir := filepath.Dir(processedPath)
	// 确保目录存在
	if err := os.MkdirAll(createDir, 0755); err != nil {
		fmt.Printf("无法创建目录: %v\n", err)
		return "", nil, fmt.Errorf("无法创建目录: %w", err)
	}

	// 4. 创建输出文件
	outFile, err := os.Create(processedPath)
	if err != nil {
		fmt.Printf("创建输出文件失败: %v\n", err)
		return "", nil, fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer outFile.Close()

	// 5. 转换为JPG格式
	options := &jpeg.Options{Quality: 90}
	if err := jpeg.Encode(outFile, img, options); err != nil {
		return "", nil, fmt.Errorf("编码JPG失败: %w", err)
	}

	// 6. 获取处理后文件大小
	stat, _ := outFile.Stat()
	processedSize := stat.Size()

	// 7. 生成元数据
	bounds := img.Bounds()
	metadata := &model.ImageMetadata{
		Width:       bounds.Dx(),
		Height:      bounds.Dy(),
		ColorSpace:  "RGB",
		Compression: "JPEG",
		DPI:         72,    // 默认DPI
		HasAlpha:    false, // JPG不支持透明度
	}

	// 8. 更新记录
	fp.dbConn.Model(record).Updates(map[string]interface{}{
		"size": processedSize,
	})

	return processedPath, metadata, nil
}

// processDocument 处理文档文件
func (fp *FileProcessor) processDocument(record *model.File) (string, *model.DocumentMetadata, error) {
	var fileNumber = strings.TrimSuffix(record.FileName, filepath.Ext(record.FileName))
	// 1. 读取原始文件
	content, err := os.ReadFile(record.OriginalFilePath)
	if err != nil {
		return "", nil, fmt.Errorf("读取文档文件失败: %w", err)
	}

	var markdownContent string
	var metadata *model.DocumentMetadata

	// 2. 根据原始格式转换为Markdown
	switch filepath.Ext(record.FileName) {
	case ".txt":
		markdownContent, metadata = fp.convertTextToMarkdown(string(content), record.FileName)
	case ".md":
		markdownContent = string(content)
		metadata = fp.extractMarkdownMetadata(markdownContent, record.FileName)
	case ".html", ".htm":
		markdownContent, metadata = fp.convertHTMLToMarkdown(string(content), record.FileName)
	default:
		return "", nil, fmt.Errorf("不支持的文档格式: %s", record.FileName)
	}

	// 3. 生成处理后的文件路径
	processedPath := filepath.Join(fp.ProcessedDir, fileNumber+".md")
	createDir := filepath.Dir(processedPath)

	// 确保目录存在
	if err := os.MkdirAll(createDir, 0755); err != nil {
		fmt.Printf("无法创建目录: %v\n", err)
		return "", nil, fmt.Errorf("无法创建目录: %w", err)
	}

	// 4. 保存Markdown文件
	if err := os.WriteFile(createDir, []byte(markdownContent), 0644); err != nil {
		return "", nil, fmt.Errorf("保存Markdown文件失败: %w", err)
	}

	// 5. 获取处理后文件大小
	stat, _ := os.Stat(processedPath)
	processedSize := stat.Size()

	// 6. 更新记录
	fp.dbConn.Model(record).Updates(map[string]interface{}{
		"size": processedSize,
	})

	return processedPath, metadata, nil
}

func (fp *FileProcessor) updateProcessError(fileID uint, err error) {
	fp.dbConn.Model(&model.File{}).Where("id = ?", fileID).Updates(map[string]interface{}{
		"error_message": err.Error(),
		"updated_at":    time.Now(),
	})
}

func (fp *FileProcessor) updateProcessSuccess(fileID uint, processedPath string, metadata interface{}) {
	metadataJSON, _ := json.Marshal(metadata)

	now := time.Now()
	fp.dbConn.Model(&model.File{}).Where("id = ?", fileID).Updates(map[string]interface{}{
		"file_path":     processedPath,
		"file_name":     filepath.Base(processedPath),
		"metadata":      metadataJSON,
		"error_message": "", // 清除之前的错误信息
		"updated_at":    now,
	})
}

func (fp *FileProcessor) convertTextToMarkdown(content, filename string) (string, *model.DocumentMetadata) {
	// 简单的文本到Markdown转换
	lines := strings.Split(content, "\n")
	var markdown strings.Builder

	// 添加标题
	title := strings.TrimSuffix(filename, filepath.Ext(filename))
	markdown.WriteString(fmt.Sprintf("# %s\n\n", title))

	// 处理内容
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			markdown.WriteString("\n")
		} else {
			markdown.WriteString(line + "\n")
		}
	}

	metadata := &model.DocumentMetadata{
		PageCount:    1,
		WordCount:    len(strings.Fields(content)),
		Language:     "unknown",
		Title:        title,
		Keywords:     []string{},
		CreationDate: time.Now().Format("2006-01-02"),
	}

	return markdown.String(), metadata
}

func (fp *FileProcessor) extractMarkdownMetadata(content, filename string) *model.DocumentMetadata {
	lines := strings.Split(content, "\n")
	wordCount := len(strings.Fields(content))

	// 提取标题（第一个#标题）
	title := strings.TrimSuffix(filename, filepath.Ext(filename))
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "# "))
			break
		}
	}

	return &model.DocumentMetadata{
		PageCount:    1,
		WordCount:    wordCount,
		Language:     "unknown",
		Title:        title,
		Keywords:     []string{},
		CreationDate: time.Now().Format("2006-01-02"),
	}
}

func (fp *FileProcessor) convertHTMLToMarkdown(content, filename string) (string, *model.DocumentMetadata) {
	// 简化的HTML到Markdown转换
	// 实际项目中建议使用专门的HTML到Markdown转换库

	// 移除HTML标签的简单实现
	content = strings.ReplaceAll(content, "<br>", "\n")
	content = strings.ReplaceAll(content, "<br/>", "\n")
	content = strings.ReplaceAll(content, "<p>", "\n")
	content = strings.ReplaceAll(content, "</p>", "\n")

	// 移除其他HTML标签（简化版本）
	var result strings.Builder
	inTag := false
	for _, char := range content {
		if char == '<' {
			inTag = true
		} else if char == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(char)
		}
	}

	cleanContent := result.String()
	title := strings.TrimSuffix(filename, filepath.Ext(filename))

	markdown := fmt.Sprintf("# %s\n\n%s", title, cleanContent)

	metadata := &model.DocumentMetadata{
		PageCount:    1,
		WordCount:    len(strings.Fields(cleanContent)),
		Language:     "unknown",
		Title:        title,
		Keywords:     []string{},
		CreationDate: time.Now().Format("2006-01-02"),
	}

	return markdown, metadata
}
