package models

import (
	"database/sql"
	"time"
)

// Person представляет человека в системе
type Person struct {
	ID        int       `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// Face представляет отдельное лицо (фотографию)
type Face struct {
	ID             int       `db:"id" json:"id"`
	PersonID       int       `db:"person_id" json:"person_id"`
	OriginalImage  string    `db:"original_image" json:"original_image"`   // Оригинальное фото
	AnnotatedImage string    `db:"annotated_image" json:"annotated_image"` // Фото с рамкой
	FaceX          int       `db:"face_x" json:"face_x"`                   // Координаты лица
	FaceY          int       `db:"face_y" json:"face_y"`
	FaceWidth      int       `db:"face_width" json:"face_width"`
	FaceHeight     int       `db:"face_height" json:"face_height"`
	Embedding      []byte    `db:"embedding" json:"-"`           // Embedding вектор
	Confidence     float64   `db:"confidence" json:"confidence"` // Уверенность детекции
	DetectedAt     time.Time `db:"detected_at" json:"detected_at"`
	ImagePath      string    `db:"image_path" json:"image_path"`
}

// Task представляет задачу обработки изображений
type Task struct {
	ID           string         `db:"id" json:"id"`
	Status       string         `db:"status" json:"status"` // processing, completed, failed
	TotalImages  int            `db:"total_images" json:"total_images"`
	TotalFaces   int            `db:"total_faces" json:"total_faces"`
	UniquPersons int            `db:"unique_persons" json:"unique_persons"`
	ErrorMessage sql.NullString `db:"error_message" json:"error_message,omitempty"`
	CreatedAt    time.Time      `db:"created_at" json:"created_at"`
	CompletedAt  sql.NullTime   `db:"completed_at" json:"completed_at,omitempty"`
}

// PersonWithFaces - человек со всеми его фотографиями
// Используется для API ответов
type PersonWithFaces struct {
	Person
	Faces []Face `json:"faces"`
	Count int    `json:"faces_count"`
}

// Stats - общая статистика системы
type Stats struct {
	TotalPersons int `json:"total_persons"`
	TotalFaces   int `json:"total_faces"`
	TotalTasks   int `json:"total_tasks"`
}

// UpdatePersonRequest - запрос на обновление имени
type UpdatePersonRequest struct {
	Name string `json:"name" binding:"required"`
}

// UploadResponse - ответ на загрузку файлов
type UploadResponse struct {
	TaskID  string `json:"task_id"`
	Message string `json:"message"`
}

// ErrorResponse - стандартный ответ с ошибкой
type ErrorResponse struct {
	Error string `json:"error"`
}

// Константы статусов задач
const (
	TaskStatusProcessing = "processing"
	TaskStatusCompleted  = "completed"
	TaskStatusFailed     = "failed"
)

// PythonResponse - ответ от Python сервера
type PythonResponse struct {
	Success       bool                    `json:"success"`
	TaskID        string                  `json:"task_id"`
	Clusters      map[string][]string     `json:"clusters"`       // person_id: [face_ids]
	Embeddings    map[string][]float64    `json:"embeddings"`     // face_id: embedding вектор
	FacesMetadata map[string]FaceMetadata `json:"faces_metadata"` // face_id: metadata
	TotalFaces    int                     `json:"total_faces"`
	UniquePersons int                     `json:"unique_persons"`
	Error         string                  `json:"error,omitempty"`
}

// FaceMetadata метаданные о лице от Python
type FaceMetadata struct {
	OriginalImage string  `json:"original_image"` // Путь к оригинальному фото
	BoxedImage    string  `json:"boxed_image"`    // Путь к фото с bbox
	Bbox          []int   `json:"bbox"`           // [x1, y1, x2, y2]
	Confidence    float64 `json:"confidence"`     // Уверенность детекции
}
