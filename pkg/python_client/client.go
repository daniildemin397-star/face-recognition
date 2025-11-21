package python_client

import (
	"bytes"
	"encoding/json"
	"face-recognition/internal/models"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Client для взаимодействия с Python сервером
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient создает новый клиент
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute, // Увеличен для InsightFace
		},
	}
}

// ProcessImages отправляет изображения на полную обработку
// Python делает: детекцию → embeddings → кластеризацию
func (c *Client) ProcessImages(imagePaths []string, taskID string, minSize int, detThresh float64) (*models.PythonResponse, error) {
	// Создаем multipart форму
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Добавляем каждое изображение
	for _, imagePath := range imagePaths {
		file, err := os.Open(imagePath)
		if err != nil {
			return nil, fmt.Errorf("не удалось открыть файл %s: %w", imagePath, err)
		}
		defer file.Close()

		part, err := writer.CreateFormFile("images", filepath.Base(imagePath))
		if err != nil {
			return nil, fmt.Errorf("ошибка создания form file: %w", err)
		}

		if _, err := io.Copy(part, file); err != nil {
			return nil, fmt.Errorf("ошибка копирования файла: %w", err)
		}
	}

	// Добавляем параметры
	writer.WriteField("task_id", taskID)
	writer.WriteField("min_size", fmt.Sprintf("%d", minSize))
	writer.WriteField("det_thresh", fmt.Sprintf("%.2f", detThresh))

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("ошибка закрытия writer: %w", err)
	}

	// Отправляем POST запрос
	resp, err := c.httpClient.Post(
		c.baseURL+"/process",
		writer.FormDataContentType(),
		body,
	)
	if err != nil {
		return nil, fmt.Errorf("ошибка HTTP запроса: %w", err)
	}
	defer resp.Body.Close()

	// Проверяем статус код
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Python вернул ошибку %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Парсим ответ
	var result models.PythonResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ошибка парсинга ответа: %w", err)
	}

	// Проверяем успешность обработки
	if !result.Success {
		return nil, fmt.Errorf("Python обработка не удалась: %s", result.Error)
	}

	return &result, nil
}

// CompareEmbeddings сравнивает два embedding
func (c *Client) CompareEmbeddings(emb1, emb2 []float64) (float64, bool, error) {
	requestBody, err := json.Marshal(map[string]interface{}{
		"embedding1": emb1,
		"embedding2": emb2,
	})
	if err != nil {
		return 0, false, err
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/compare",
		"application/json",
		bytes.NewBuffer(requestBody),
	)
	if err != nil {
		return 0, false, err
	}
	defer resp.Body.Close()

	var result struct {
		Similarity float64 `json:"similarity"`
		Match      bool    `json:"match"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, false, err
	}

	return result.Similarity, result.Match, nil
}

// HealthCheck проверяет доступность Python сервера
func (c *Client) HealthCheck() error {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("Python сервер недоступен: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Python сервер вернул статус %d", resp.StatusCode)
	}

	return nil
}
