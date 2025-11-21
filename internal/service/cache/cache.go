package cache

import (
	"context"
	"encoding/json"
	"face-recognition/internal/models"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Service управляет кэшированием через Redis
type Service struct {
	client *redis.Client
	ctx    context.Context
}

// NewService создает новый cache service
func NewService(addr, password string, db int) (*Service, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()

	// Проверяем подключение
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("не удалось подключиться к Redis: %w", err)
	}

	return &Service{
		client: client,
		ctx:    ctx,
	}, nil
}

// Close закрывает соединение с Redis
func (s *Service) Close() error {
	return s.client.Close()
}

// ============ PERSON CACHE ============

// GetPerson получает персону из кэша
func (s *Service) GetPerson(id int) (*models.PersonWithFaces, error) {
	key := fmt.Sprintf("person:%d", id)

	data, err := s.client.Get(s.ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil // Не найдено в кэше
	}
	if err != nil {
		return nil, err
	}

	var person models.PersonWithFaces
	if err := json.Unmarshal(data, &person); err != nil {
		return nil, err
	}

	return &person, nil
}

// SetPerson сохраняет персону в кэш на 1 час
func (s *Service) SetPerson(person *models.PersonWithFaces) error {
	key := fmt.Sprintf("person:%d", person.ID)

	data, err := json.Marshal(person)
	if err != nil {
		return err
	}

	return s.client.Set(s.ctx, key, data, 1*time.Hour).Err()
}

// InvalidatePerson удаляет персону из кэша
func (s *Service) InvalidatePerson(id int) error {
	key := fmt.Sprintf("person:%d", id)
	return s.client.Del(s.ctx, key).Err()
}

// ============ TASK CACHE ============

// GetTask получает задачу из кэша
func (s *Service) GetTask(taskID string) (*models.Task, error) {
	key := fmt.Sprintf("task:%s", taskID)

	data, err := s.client.Get(s.ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var task models.Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, err
	}

	return &task, nil
}

// SetTask сохраняет задачу в кэш
func (s *Service) SetTask(task *models.Task) error {
	key := fmt.Sprintf("task:%s", task.ID)

	data, err := json.Marshal(task)
	if err != nil {
		return err
	}

	// Задачи храним 24 часа
	return s.client.Set(s.ctx, key, data, 24*time.Hour).Err()
}

// ============ STATS CACHE ============

// GetStats получает статистику из кэша
func (s *Service) GetStats() (*models.Stats, error) {
	data, err := s.client.Get(s.ctx, "stats").Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var stats models.Stats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, err
	}

	return &stats, nil
}

// SetStats сохраняет статистику в кэш на 5 минут
func (s *Service) SetStats(stats *models.Stats) error {
	data, err := json.Marshal(stats)
	if err != nil {
		return err
	}

	return s.client.Set(s.ctx, "stats", data, 5*time.Minute).Err()
}

// InvalidateStats очищает кэш статистики
func (s *Service) InvalidateStats() error {
	return s.client.Del(s.ctx, "stats").Err()
}

// ============ EMBEDDINGS CACHE ============

// GetEmbedding получает embedding для изображения
func (s *Service) GetEmbedding(imagePath string) ([]float64, error) {
	key := fmt.Sprintf("embedding:%s", imagePath)

	data, err := s.client.Get(s.ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var embedding []float64
	if err := json.Unmarshal(data, &embedding); err != nil {
		return nil, err
	}

	return embedding, nil
}

// SetEmbedding сохраняет embedding в кэш на 7 дней
func (s *Service) SetEmbedding(imagePath string, embedding []float64) error {
	key := fmt.Sprintf("embedding:%s", imagePath)

	data, err := json.Marshal(embedding)
	if err != nil {
		return err
	}

	return s.client.Set(s.ctx, key, data, 7*24*time.Hour).Err()
}

// ============ UTILITY ============

// FlushAll очищает весь кэш (только для разработки!)
func (s *Service) FlushAll() error {
	return s.client.FlushAll(s.ctx).Err()
}
