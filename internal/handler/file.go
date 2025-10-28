package handlers

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Planckbaka/go-backend/internal/database"
	"github.com/Planckbaka/go-backend/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func UploadMultipleFiles(c *gin.Context) {

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "无法解析表单数据",
			"detail": err.Error(),
		})
		return
	}

	files := form.File["files"] //
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "no files",
		})
		return
	}

	// 用于存储成功上传的文件信息
	var uploadedFiles []model.File
	var failedFiles []string

	for _, file := range files {
		// Upload the file to specific dst.
		fileRecord, err := processSingleFile(file)
		if err != nil {
			failedFiles = append(failedFiles, fmt.Sprintf("%s: %v", file.Filename, err))
			continue
		}
		uploadedFiles = append(uploadedFiles, *fileRecord)

	}
	response := gin.H{
		"success": len(uploadedFiles),
		"failed":  len(failedFiles),
		"files":   uploadedFiles,
	}

	if len(failedFiles) > 0 {
		response["errors"] = failedFiles
	}

	statusCode := http.StatusOK
	if len(uploadedFiles) == 0 {
		statusCode = http.StatusBadRequest
	}

	c.JSON(statusCode, response)
}

func processSingleFile(fileHeader *multipart.FileHeader) (*model.File, error) {

	dbConn := database.DB
	uploadDir := "uploads/original"
	// 打开上传的文件
	src, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("无法打开文件: %w", err)
	}
	defer func(src multipart.File) {
		err := src.Close()
		if err != nil {

		}
	}(src)
	dateDir := time.Now().Format("2006/01/02")
	//join them
	fullDir := filepath.Join(uploadDir, dateDir)
	createDir := filepath.Join(".", fullDir)

	// 确保目录存在
	if err := os.MkdirAll(createDir, 0755); err != nil {
		return nil, fmt.Errorf("无法创建目录: %w", err)
	}

	fileExt := filepath.Ext(fileHeader.Filename)
	fileNumber, err := getNextFileNumber(dbConn, fullDir)
	if err != nil {
		return nil, err
	}
	// create new file name like:1.jpg, 2.png, 3.pdf
	newFilename := fmt.Sprintf("%d%s", fileNumber, fileExt)
	// join
	filePath := filepath.Join(fullDir, newFilename)

	dst, err := os.Create(filePath)
	if err != nil {
		return nil, err
	}
	defer func(dst *os.File) {
		err := dst.Close()
		if err != nil {

		}
	}(dst)

	if _, err := io.Copy(dst, src); err != nil {
		return nil, err
	}

	// 第五步：准备数据库记录
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	var fileRecord *model.File

	fileRecord = &model.File{
		ContentType:      contentType,
		FileName:         newFilename,
		OriginalFilePath: filePath,
		Size:             fileHeader.Size,
	}

	// 第六步：在同一事务中保存数据库记录
	// 这一步很关键：只有当数据库保存成功，事务才会提交
	// 如果这里失败，整个事务回滚，但文件已经写入磁盘了
	// 所以我们需要在事务失败后清理文件
	if err := dbConn.Create(fileRecord).Error; err != nil {
		return nil, err
	}
	return fileRecord, nil
}

func getNextFileNumber(tx *gorm.DB, fullPath string) (int, error) {
	var maxFile model.File

	pathPattern := filepath.Join(fullPath, "%")
	fmt.Println(pathPattern)

	err := tx.Where("original_file_path LIKE ?", pathPattern).
		Order("id DESC").
		Limit(1).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&maxFile).Error

	// 如果没找到记录（今天第一个文件），从 1 开始
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("查询最大编号失败: %w", err)
	}
	// 从文件路径中提取当前编号
	// 例如：uploads/original/2025/10/06/5.jpg -> 5
	fileName := filepath.Base(maxFile.OriginalFilePath)
	ext := filepath.Ext(fileName)
	numberStr := fileName[:len(fileName)-len(ext)]

	currentNumber, err := strconv.Atoi(numberStr)
	if err != nil {
		// 如果解析失败，可能是文件名格式不对，默认从 1 开始
		return 1, nil
	}

	// 返回下一个编号
	return currentNumber + 1, nil
}
