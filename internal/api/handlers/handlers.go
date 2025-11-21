package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"face-recognition/internal/api/websocket"
	"face-recognition/internal/models"
	"face-recognition/internal/repository"
	"face-recognition/internal/service/cache"
	"face-recognition/internal/service/storage"
	"face-recognition/pkg/python_client"

	"github.com/gin-gonic/gin"
)

// Handler содержит все зависимости для обработки HTTP запросов
type Handler struct {
	repo         repository.RepositoryInterface
	storage      *storage.Service
	pythonClient *python_client.Client
	cache        *cache.Service
	wsManager    *websocket.Manager
}

// NewHandler создает новый handler с зависимостями
func NewHandler(
	repo repository.RepositoryInterface,
	storage *storage.Service,
	pythonClient *python_client.Client,
	cache *cache.Service,
	wsManager *websocket.Manager,
) *Handler {
	return &Handler{
		repo:         repo,
		storage:      storage,
		pythonClient: pythonClient,
		cache:        cache,
		wsManager:    wsManager,
	}
}

// ============ UPLOAD ============

// HandleUpload обрабатывает загрузку файлов
func (h *Handler) HandleUpload(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Ошибка получения файлов",
		})
		return
	}

	files := form.File["images"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Файлы не загружены",
		})
		return
	}

	// Сохраняем файлы через storage service
	taskID, savedFiles, err := h.storage.SaveUploadedFiles(files)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: fmt.Sprintf("Ошибка сохранения файлов: %v", err),
		})
		return
	}

	// Создаем задачу в БД
	if err := h.repo.CreateTask(taskID, len(files)); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "Ошибка создания задачи",
		})
		return
	}

	// Запускаем обработку асинхронно
	go h.processImages(taskID, savedFiles)

	c.JSON(http.StatusOK, models.UploadResponse{
		TaskID:  taskID,
		Message: fmt.Sprintf("Загружено %d файлов, начата обработка", len(files)),
	})
}

// processImages обрабатывает изображения через Python (InsightFace)
func (h *Handler) processImages(taskID string, imagePaths []string) {
	// Отправляем начальное уведомление
	h.wsManager.BroadcastTaskUpdate(taskID, models.TaskStatusProcessing, map[string]interface{}{
		"message": "Начало обработки",
		"total":   len(imagePaths),
	})

	log.Printf("🚀 Задача %s: Обработка %d изображений", taskID, len(imagePaths))

	// Этап 1: Отправка в Python (детекция + embeddings + кластеризация)
	h.wsManager.BroadcastTaskProgress(taskID, 10, 100, "Отправка в Python")

	// Параметры детекции
	minSize := 30    // Минимальный размер лица в пикселях
	detThresh := 0.5 // Порог уверенности детекции

	// Вызываем Python для полной обработки
	result, err := h.pythonClient.ProcessImages(imagePaths, taskID, minSize, detThresh)

	if err != nil {
		errorMsg := fmt.Sprintf("Ошибка Python обработки: %v", err)
		log.Printf("❌ %s", errorMsg)
		h.repo.UpdateTaskStatus(taskID, models.TaskStatusFailed, &errorMsg)

		h.wsManager.BroadcastTaskUpdate(taskID, models.TaskStatusFailed, map[string]interface{}{
			"error": errorMsg,
		})
		return
	}

	log.Printf("✅ Python обработка завершена: %d лиц, %d людей", result.TotalFaces, result.UniquePersons)

	// Этап 2: Сохранение результатов в БД
	h.wsManager.BroadcastTaskProgress(taskID, 70, 100, "Сохранение в базу данных")

	totalFaces := 0
	uniquePersons := 0

	// Обрабатываем каждый кластер
	for clusterID, faceIDs := range result.Clusters {
		// Пропускаем noise кластер
		if clusterID == "noise" {
			log.Printf("⚠️  Пропускаем %d outlier лиц", len(faceIDs))
			continue
		}

		// Создаем или находим персону
		personID, err := h.repo.GetOrCreatePerson(clusterID)
		if err != nil {
			log.Printf("⚠️  Ошибка создания персоны %s: %v", clusterID, err)
			continue
		}
		uniquePersons++

		// Сохраняем каждое лицо в кластере
		for _, faceID := range faceIDs {
			// Получаем метаданные лица
			metadata, exists := result.FacesMetadata[faceID]
			if !exists {
				log.Printf("⚠️  Метаданные для %s не найдены", faceID)
				continue
			}

			// Получаем embedding
			embedding, exists := result.Embeddings[faceID]
			if !exists {
				log.Printf("⚠️  Embedding для %s не найден", faceID)
				continue
			}

			// Конвертируем embedding в JSON для хранения
			embeddingBytes, err := json.Marshal(embedding)
			if err != nil {
				log.Printf("⚠️  Ошибка сериализации embedding: %v", err)
				continue
			}

			// Вычисляем координаты bbox
			// bbox от Python: [x1, y1, x2, y2]
			var faceX, faceY, faceWidth, faceHeight int
			if len(metadata.Bbox) == 4 {
				faceX = metadata.Bbox[0]
				faceY = metadata.Bbox[1]
				faceWidth = metadata.Bbox[2] - metadata.Bbox[0]
				faceHeight = metadata.Bbox[3] - metadata.Bbox[1]
			}

			// Создаем запись лица в БД
			face := &models.Face{
				PersonID:       personID,
				OriginalImage:  metadata.OriginalImage,
				AnnotatedImage: metadata.BoxedImage,
				FaceX:          faceX,
				FaceY:          faceY,
				FaceWidth:      faceWidth,
				FaceHeight:     faceHeight,
				Embedding:      embeddingBytes,
				Confidence:     metadata.Confidence,
			}

			if err := h.repo.CreateFace(face); err != nil {
				log.Printf("⚠️  Ошибка сохранения лица в БД: %v", err)
				log.Printf("   Face data: PersonID=%d, OriginalImage=%s, AnnotatedImage=%s",
					face.PersonID, face.OriginalImage, face.AnnotatedImage)
				continue
			}
			totalFaces++

			log.Printf("   ✓ Сохранено лицо %s: PersonID=%d, bbox=(%d,%d,%dx%d)",
				faceID, personID, faceX, faceY, faceWidth, faceHeight)
		}
	}

	log.Printf("💾 Сохранено в БД: %d лиц, %d людей", totalFaces, uniquePersons)

	// Обновляем статистику задачи
	h.repo.UpdateTaskStats(taskID, totalFaces, uniquePersons)
	h.repo.UpdateTaskStatus(taskID, models.TaskStatusCompleted, nil)

	// Инвалидируем кэш
	if h.cache != nil {
		h.cache.InvalidateStats()
	}

	// Отправляем финальное уведомление
	h.wsManager.BroadcastTaskUpdate(taskID, models.TaskStatusCompleted, map[string]interface{}{
		"total_faces":    totalFaces,
		"unique_persons": uniquePersons,
	})

	h.wsManager.BroadcastTaskProgress(taskID, 100, 100, "Готово!")

	// Обновляем статистику для всех клиентов
	if stats, err := h.repo.GetStats(); err == nil {
		h.wsManager.BroadcastStatsUpdate(stats)
	}

	log.Printf("✅ Задача %s завершена успешно", taskID)
}

// ============ TASKS ============

// HandleTaskStatus возвращает статус задачи (с кэшем)
func (h *Handler) HandleTaskStatus(c *gin.Context) {
	taskID := c.Param("id")

	// Пробуем из кэша
	if h.cache != nil {
		if task, err := h.cache.GetTask(taskID); err == nil && task != nil {
			c.JSON(http.StatusOK, task)
			return
		}
	}

	// Из БД
	task, err := h.repo.GetTask(taskID)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "Задача не найдена",
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: err.Error(),
		})
		return
	}

	// Сохраняем в кэш
	if h.cache != nil {
		h.cache.SetTask(task)
	}

	c.JSON(http.StatusOK, task)
}

// ============ PERSONS ============

// HandleGetPersons возвращает всех людей
func (h *Handler) HandleGetPersons(c *gin.Context) {
	persons, err := h.repo.GetAllPersons()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: err.Error(),
		})
		return
	}

	if persons == nil {
		persons = []models.PersonWithFaces{}
	}

	c.JSON(http.StatusOK, persons)
}

// HandleGetPerson возвращает конкретного человека со всеми фото (с кэшем)
func (h *Handler) HandleGetPerson(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Неверный ID",
		})
		return
	}

	// Пробуем из кэша
	if h.cache != nil {
		if person, err := h.cache.GetPerson(id); err == nil && person != nil {
			c.JSON(http.StatusOK, person)
			return
		}
	}

	// Из БД
	person, err := h.repo.GetPersonByID(id)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "Человек не найден",
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: err.Error(),
		})
		return
	}

	// Сохраняем в кэш
	if h.cache != nil {
		h.cache.SetPerson(person)
	}

	c.JSON(http.StatusOK, person)
}

// HandleUpdatePerson обновляет имя человека
func (h *Handler) HandleUpdatePerson(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Неверный ID",
		})
		return
	}

	var req models.UpdatePersonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Имя обязательно",
		})
		return
	}

	err = h.repo.UpdatePersonName(id, req.Name)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "Человек не найден",
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: err.Error(),
		})
		return
	}

	// Инвалидируем кэш
	if h.cache != nil {
		h.cache.InvalidatePerson(id)
		h.cache.InvalidateStats()
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Имя обновлено",
		"name":    req.Name,
	})
}

// HandleDeletePerson удаляет человека
func (h *Handler) HandleDeletePerson(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Неверный ID",
		})
		return
	}

	// Удаляем из БД и получаем список файлов для удаления
	faces, err := h.repo.DeletePerson(id)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "Человек не найден",
		})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: err.Error(),
		})
		return
	}

	// Удаляем файлы (original и annotated)
	var paths []string
	for _, face := range faces {
		paths = append(paths, face.OriginalImage)
		if face.AnnotatedImage != "" {
			paths = append(paths, face.AnnotatedImage)
		}
	}
	h.storage.DeleteFiles(paths)

	// Инвалидируем кэш
	if h.cache != nil {
		h.cache.InvalidatePerson(id)
		h.cache.InvalidateStats()
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Человек удален",
	})
}

// ============ SEARCH ============

// HandleSearch ищет людей по имени или ID
func (h *Handler) HandleSearch(c *gin.Context) {
	query := c.Query("q")

	if query == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "Параметр q обязателен",
		})
		return
	}

	persons, err := h.repo.SearchPersons(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: err.Error(),
		})
		return
	}

	if persons == nil {
		persons = []models.PersonWithFaces{}
	}

	c.JSON(http.StatusOK, persons)
}

// ============ STATS ============

// HandleGetStats возвращает общую статистику (с кэшем)
func (h *Handler) HandleGetStats(c *gin.Context) {
	// Пробуем из кэша
	if h.cache != nil {
		if stats, err := h.cache.GetStats(); err == nil && stats != nil {
			c.JSON(http.StatusOK, stats)
			return
		}
	}

	// Из БД
	stats, err := h.repo.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: err.Error(),
		})
		return
	}

	// Сохраняем в кэш
	if h.cache != nil {
		h.cache.SetStats(stats)
	}

	c.JSON(http.StatusOK, stats)
}
