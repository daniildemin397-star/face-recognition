package repository

import "face-recognition/internal/models"

// RepositoryInterface определяет контракт для работы с данными
// Это позволяет легко мокать репозиторий в тестах
type RepositoryInterface interface {
	// Tasks
	CreateTask(taskID string, totalImages int) error
	GetTask(taskID string) (*models.Task, error)
	UpdateTaskStatus(taskID, status string, errorMsg *string) error
	UpdateTaskStats(taskID string, totalFaces, uniquePersons int) error

	// Persons
	GetOrCreatePerson(name string) (int, error)
	GetAllPersons() ([]models.PersonWithFaces, error)
	GetPersonByID(id int) (*models.PersonWithFaces, error)
	UpdatePersonName(id int, name string) error
	DeletePerson(id int) ([]models.Face, error)
	SearchPersons(query string) ([]models.PersonWithFaces, error)

	// Faces
	CreateFace(face *models.Face) error

	// Stats
	GetStats() (*models.Stats, error)
}

// Проверяем что Repository реализует RepositoryInterface
var _ RepositoryInterface = (*Repository)(nil)
