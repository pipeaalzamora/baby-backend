package handlers

import (
	"errors"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"time"
)

const maxImageUploadBytes int64 = 20 * 1024 * 1024

var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

type presignMediaRequest struct {
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
	Date        string `json:"date"`
}

type presignMediaResponse struct {
	UploadURL   string            `json:"uploadUrl"`
	Bucket      string            `json:"bucket"`
	Key         string            `json:"key"`
	ContentType string            `json:"contentType"`
	ExpiresAt   time.Time         `json:"expiresAt"`
	Headers     map[string]string `json:"headers"`
}

func normalizeImageUpload(fileName, contentType string, sizeBytes int64) (string, string, error) {
	if sizeBytes <= 0 {
		return "", "", errors.New("tamaño de archivo inválido")
	}
	if sizeBytes > maxImageUploadBytes {
		return "", "", errors.New("imagen demasiado grande, máximo 20 MB")
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if contentType == "" && ext != "" {
		contentType = strings.ToLower(mime.TypeByExtension(ext))
	}
	if contentType == "image/jpg" || contentType == "image/pjpeg" {
		contentType = "image/jpeg"
	}

	expectedExt, ok := allowedImageTypes[contentType]
	if !ok {
		return "", "", errors.New("solo se permiten imágenes jpg, png, webp o gif")
	}
	return contentType, expectedExt, nil
}

func photoObjectKey(userID, childID, date, fileName, contentType string) (string, error) {
	contentType, ext, err := normalizeImageUpload(fileName, contentType, 1)
	if err != nil {
		return "", err
	}
	_ = contentType

	takenAt, err := time.Parse("2006-01-02", strings.TrimSpace(date))
	if err != nil {
		takenAt = time.Now()
	}

	return fmt.Sprintf(
		"accounts/%s/children/%s/photos/%04d/%02d/%02d/%s%s",
		safePathSegment(userID),
		safePathSegment(childID),
		takenAt.Year(),
		takenAt.Month(),
		takenAt.Day(),
		randomHex(16),
		ext,
	), nil
}

func childProfileObjectKey(userID, childID, fileName, contentType string) (string, error) {
	contentType, ext, err := normalizeImageUpload(fileName, contentType, 1)
	if err != nil {
		return "", err
	}
	_ = contentType

	return fmt.Sprintf(
		"accounts/%s/children/%s/profile/%s%s",
		safePathSegment(userID),
		safePathSegment(childID),
		randomHex(16),
		ext,
	), nil
}

func s3ChildFolderPrefix(userID, childID, folder string) string {
	return fmt.Sprintf(
		"accounts/%s/children/%s/%s/",
		safePathSegment(userID),
		safePathSegment(childID),
		safePathSegment(folder),
	)
}

func isS3ChildFolderKey(userID, childID, folder, key string) bool {
	key = strings.TrimSpace(key)
	return key != "" && strings.HasPrefix(key, s3ChildFolderPrefix(userID, childID, folder))
}

func safePathSegment(value string) string {
	value = strings.TrimSpace(value)
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-", "..", "-")
	value = replacer.Replace(value)
	if value == "" {
		return "unknown"
	}
	return value
}
