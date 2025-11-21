package handlers

import (
	"bytes"
	"encoding/json"
	"face-recognition/internal/models"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRepository - мок репозитория для тестов
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) GetStats() (*models.Stats, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Stats), args.Error(1)
}

func (m *MockRepository) GetAllPersons() ([]models.PersonWithFaces, error) {
	args := m.Called()
	return args.Get(0).([]models.PersonWithFaces), args.Error(1)
}

func (m *MockRepository) GetPersonByID(id int) (*models.PersonWithFaces, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PersonWithFaces), args.Error(1)
}

func (m *MockRepository) UpdatePersonName(id int, name string) error {
	args := m.Called(id, name)
	return args.Error(0)
}

func (m *MockRepository) SearchPersons(query string) ([]models.PersonWithFaces, error) {
	args := m.Called(query)
	return args.Get(0).([]models.PersonWithFaces), args.Error(1)
}

// Остальные методы для полноты интерфейса
func (m *MockRepository) CreateTask(taskID string, totalImages int) error {
	args := m.Called(taskID, totalImages)
	return args.Error(0)
}

func (m *MockRepository) GetTask(taskID string) (*models.Task, error) {
	args := m.Called(taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockRepository) UpdateTaskStatus(taskID, status string, errorMsg *string) error {
	args := m.Called(taskID, status, errorMsg)
	return args.Error(0)
}

func (m *MockRepository) UpdateTaskStats(taskID string, totalFaces, uniquePersons int) error {
	args := m.Called(taskID, totalFaces, uniquePersons)
	return args.Error(0)
}

func (m *MockRepository) GetOrCreatePerson(name string) (int, error) {
	args := m.Called(name)
	return args.Int(0), args.Error(1)
}

func (m *MockRepository) DeletePerson(id int) ([]models.Face, error) {
	args := m.Called(id)
	return args.Get(0).([]models.Face), args.Error(1)
}

func (m *MockRepository) CreateFace(personID int, imagePath string, embedding []float64, confidence float64) error {
	args := m.Called(personID, imagePath, embedding, confidence)
	return args.Error(0)
}

func (m *MockRepository) SaveFacesTransaction(clusters map[string][]string, embeddings map[string][]float64) (int, int, error) {
	args := m.Called(clusters, embeddings)
	return args.Int(0), args.Int(1), args.Error(2)
}

// setupTestRouter создает тестовый роутер
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

func TestHandleGetStats(t *testing.T) {
	// Setup
	mockRepo := new(MockRepository)
	handler := &Handler{repo: mockRepo}

	expectedStats := &models.Stats{
		TotalPersons: 10,
		TotalFaces:   50,
		TotalTasks:   5,
	}

	mockRepo.On("GetStats").Return(expectedStats, nil)

	// Создаем тестовый запрос
	router := setupTestRouter()
	router.GET("/stats", handler.HandleGetStats)

	req, _ := http.NewRequest("GET", "/stats", nil)
	w := httptest.NewRecorder()

	// Выполняем запрос
	router.ServeHTTP(w, req)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var stats models.Stats
	err := json.Unmarshal(w.Body.Bytes(), &stats)
	assert.NoError(t, err)
	assert.Equal(t, 10, stats.TotalPersons)
	assert.Equal(t, 50, stats.TotalFaces)
	assert.Equal(t, 5, stats.TotalTasks)

	mockRepo.AssertExpectations(t)
}

func TestHandleGetPersons(t *testing.T) {
	mockRepo := new(MockRepository)
	handler := &Handler{repo: mockRepo}

	expectedPersons := []models.PersonWithFaces{
		{
			Person: models.Person{ID: 1, Name: "John Doe"},
			Count:  3,
		},
		{
			Person: models.Person{ID: 2, Name: "Jane Smith"},
			Count:  5,
		},
	}

	mockRepo.On("GetAllPersons").Return(expectedPersons, nil)

	router := setupTestRouter()
	router.GET("/persons", handler.HandleGetPersons)

	req, _ := http.NewRequest("GET", "/persons", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var persons []models.PersonWithFaces
	err := json.Unmarshal(w.Body.Bytes(), &persons)
	assert.NoError(t, err)
	assert.Len(t, persons, 2)
	assert.Equal(t, "John Doe", persons[0].Name)
	assert.Equal(t, 3, persons[0].Count)

	mockRepo.AssertExpectations(t)
}

func TestHandleUpdatePerson(t *testing.T) {
	mockRepo := new(MockRepository)
	handler := &Handler{repo: mockRepo}

	newName := "Updated Name"
	mockRepo.On("UpdatePersonName", 1, newName).Return(nil)

	router := setupTestRouter()
	router.PUT("/persons/:id", handler.HandleUpdatePerson)

	reqBody := models.UpdatePersonRequest{Name: newName}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("PUT", "/persons/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Имя обновлено", response["message"])
	assert.Equal(t, newName, response["name"])

	mockRepo.AssertExpectations(t)
}

func TestHandleSearch(t *testing.T) {
	mockRepo := new(MockRepository)
	handler := &Handler{repo: mockRepo}

	query := "John"
	expectedResults := []models.PersonWithFaces{
		{
			Person: models.Person{ID: 1, Name: "John Doe"},
			Count:  3,
		},
	}

	mockRepo.On("SearchPersons", query).Return(expectedResults, nil)

	router := setupTestRouter()
	router.GET("/search", handler.HandleSearch)

	req, _ := http.NewRequest("GET", "/search?q="+query, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var results []models.PersonWithFaces
	err := json.Unmarshal(w.Body.Bytes(), &results)
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "John Doe", results[0].Name)

	mockRepo.AssertExpectations(t)
}

func TestHandleSearchMissingQuery(t *testing.T) {
	mockRepo := new(MockRepository)
	handler := &Handler{repo: mockRepo}

	router := setupTestRouter()
	router.GET("/search", handler.HandleSearch)

	req, _ := http.NewRequest("GET", "/search", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Параметр q обязателен", response.Error)
}
