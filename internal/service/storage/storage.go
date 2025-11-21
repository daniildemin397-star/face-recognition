package storage

import (
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// Service управляет файловым хранилищем
type Service struct {
	uploadsDir string
	resultsDir string
}

// NewService создает новый файловый сервис
func NewService(uploadsDir, resultsDir string) (*Service, error) {
	// Создаем директории если их нет
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		return nil, fmt.Errorf("не удалось создать uploads: %w", err)
	}

	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return nil, fmt.Errorf("не удалось создать results: %w", err)
	}

	return &Service{
		uploadsDir: uploadsDir,
		resultsDir: resultsDir,
	}, nil
}

// SaveUploadedFiles сохраняет загруженные файлы
// Возвращает taskID и список путей к сохраненным файлам
func (s *Service) SaveUploadedFiles(files []*multipart.FileHeader) (string, []string, error) {
	// Генерируем уникальный ID задачи
	taskID := uuid.New().String()
	taskDir := filepath.Join(s.uploadsDir, taskID)

	// Создаем папку для задачи
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		return "", nil, fmt.Errorf("не удалось создать папку задачи: %w", err)
	}

	var savedFiles []string

	// Сохраняем каждый файл
	for _, fileHeader := range files {
		// Открываем загруженный файл
		file, err := fileHeader.Open()
		if err != nil {
			return "", nil, fmt.Errorf("не удалось открыть файл %s: %w", fileHeader.Filename, err)
		}
		defer file.Close()

		// Путь для сохранения
		destPath := filepath.Join(taskDir, fileHeader.Filename)

		// Создаем файл на диске
		destFile, err := os.Create(destPath)
		if err != nil {
			return "", nil, fmt.Errorf("не удалось создать файл %s: %w", destPath, err)
		}
		defer destFile.Close()

		// Копируем содержимое
		if _, err := destFile.ReadFrom(file); err != nil {
			return "", nil, fmt.Errorf("ошибка записи файла %s: %w", destPath, err)
		}

		savedFiles = append(savedFiles, destPath)
	}

	return taskID, savedFiles, nil
}

// DeleteFiles удаляет файлы по путям
func (s *Service) DeleteFiles(paths []string) error {
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("не удалось удалить %s: %w", path, err)
		}
	}
	return nil
}

// DeleteTaskDirectory удаляет всю папку задачи
func (s *Service) DeleteTaskDirectory(taskID string) error {
	taskDir := filepath.Join(s.uploadsDir, taskID)
	return os.RemoveAll(taskDir)
}

// GetUploadPath возвращает путь к папке uploads
func (s *Service) GetUploadPath(taskID, filename string) string {
	return filepath.Join(s.uploadsDir, taskID, filename)
}

// FileExists проверяет существование файла
func (s *Service) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetFileSize возвращает размер файла в байтах
func (s *Service) GetFileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// CleanupOldTasks удаляет старые задачи (можно вызывать по cron)
// age - возраст в часах
func (s *Service) CleanupOldTasks(ageHours int) error {
	// Получаем все папки в uploads
	entries, err := os.ReadDir(s.uploadsDir)
	if err != nil {
		return err
	}

	// Проверяем каждую папку
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirPath := filepath.Join(s.uploadsDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Если папка старше указанного возраста - удаляем
		// (для продакшена лучше проверять по БД)
		// age := time.Since(info.ModTime()).Hours()
		// if age > float64(ageHours) {
		//     os.RemoveAll(dirPath)
		// }

		_ = info // Пока не используем
		_ = dirPath
	}

	return nil
}
