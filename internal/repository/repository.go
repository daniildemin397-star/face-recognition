package repository

import (
	"database/sql"
	"face-recognition/internal/models"

	"github.com/jmoiron/sqlx"
)

// Repository инкапсулирует всю работу с базой данных
type Repository struct {
	db *sqlx.DB
}

// NewRepository создает новый репозиторий
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// ============ TASKS ============

// CreateTask создает новую задачу обработки
func (r *Repository) CreateTask(taskID string, totalImages int) error {
	_, err := r.db.Exec(`
		INSERT INTO tasks (id, status, total_images, created_at) 
		VALUES ($1, $2, $3, NOW())
	`, taskID, models.TaskStatusProcessing, totalImages)
	return err
}

// GetTask получает задачу по ID
func (r *Repository) GetTask(taskID string) (*models.Task, error) {
	var task models.Task
	err := r.db.Get(&task, "SELECT * FROM tasks WHERE id = $1", taskID)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// UpdateTaskStatus обновляет статус задачи
func (r *Repository) UpdateTaskStatus(taskID, status string, errorMsg *string) error {
	if errorMsg != nil {
		_, err := r.db.Exec(`
			UPDATE tasks 
			SET status = $1, error_message = $2, completed_at = NOW() 
			WHERE id = $3
		`, status, *errorMsg, taskID)
		return err
	}

	_, err := r.db.Exec(`
		UPDATE tasks 
		SET status = $1, completed_at = NOW() 
		WHERE id = $2
	`, status, taskID)
	return err
}

// UpdateTaskStats обновляет статистику задачи
func (r *Repository) UpdateTaskStats(taskID string, totalFaces, uniquePersons int) error {
	_, err := r.db.Exec(`
		UPDATE tasks 
		SET total_faces = $1, unique_persons = $2 
		WHERE id = $3
	`, totalFaces, uniquePersons, taskID)
	return err
}

// ============ PERSONS ============

// GetOrCreatePerson получает или создает персону по имени
func (r *Repository) GetOrCreatePerson(name string) (int, error) {
	var personID int

	// Пробуем найти существующую
	err := r.db.QueryRow(`
		SELECT id FROM persons WHERE name = $1
	`, name).Scan(&personID)

	// Если не найдена - создаем
	if err == sql.ErrNoRows {
		err = r.db.QueryRow(`
			INSERT INTO persons (name) 
			VALUES ($1) 
			RETURNING id
		`, name).Scan(&personID)
	}

	return personID, err
}

// GetAllPersons возвращает всех людей с количеством фото
func (r *Repository) GetAllPersons() ([]models.PersonWithFaces, error) {
	rows, err := r.db.Query(`
		SELECT p.id, p.name, p.created_at, p.updated_at, COUNT(f.id) as faces_count
		FROM persons p
		LEFT JOIN faces f ON p.id = f.person_id
		GROUP BY p.id
		ORDER BY p.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var persons []models.PersonWithFaces
	for rows.Next() {
		var p models.PersonWithFaces
		err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt, &p.UpdatedAt, &p.Count)
		if err != nil {
			continue
		}
		persons = append(persons, p)
	}

	return persons, nil
}

// GetPersonByID получает человека по ID со всеми фото
func (r *Repository) GetPersonByID(id int) (*models.PersonWithFaces, error) {
	var person models.PersonWithFaces

	// Получаем персону
	err := r.db.Get(&person.Person, "SELECT * FROM persons WHERE id = $1", id)
	if err != nil {
		return nil, err
	}

	// Получаем все фото
	err = r.db.Select(&person.Faces, `
		SELECT id, person_id, original_image, annotated_image, 
		       face_x, face_y, face_width, face_height,
		       embedding, confidence, detected_at 
		FROM faces 
		WHERE person_id = $1 
		ORDER BY detected_at DESC
	`, id)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if person.Faces == nil {
		person.Faces = []models.Face{} // Пустой массив вместо nil
	}

	person.Count = len(person.Faces)
	return &person, nil
}

// UpdatePersonName обновляет имя человека
func (r *Repository) UpdatePersonName(id int, name string) error {
	result, err := r.db.Exec(`
		UPDATE persons 
		SET name = $1, updated_at = NOW() 
		WHERE id = $2
	`, name, id)

	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// DeletePerson удаляет человека (faces удалятся автоматически через CASCADE)
func (r *Repository) DeletePerson(id int) ([]models.Face, error) {
	// Сначала получаем все фото для удаления файлов
	var faces []models.Face
	r.db.Select(&faces, "SELECT * FROM faces WHERE person_id = $1", id)

	// Удаляем из БД
	result, err := r.db.Exec("DELETE FROM persons WHERE id = $1", id)
	if err != nil {
		return nil, err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, sql.ErrNoRows
	}

	return faces, nil
}

// SearchPersons ищет людей по имени или ID
func (r *Repository) SearchPersons(query string) ([]models.PersonWithFaces, error) {
	rows, err := r.db.Query(`
		SELECT p.id, p.name, p.created_at, p.updated_at, COUNT(f.id) as faces_count
		FROM persons p
		LEFT JOIN faces f ON p.id = f.person_id
		WHERE p.name ILIKE $1 OR CAST(p.id AS TEXT) = $2
		GROUP BY p.id
		ORDER BY p.created_at DESC
	`, "%"+query+"%", query)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var persons []models.PersonWithFaces
	for rows.Next() {
		var p models.PersonWithFaces
		err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt, &p.UpdatedAt, &p.Count)
		if err != nil {
			continue
		}
		persons = append(persons, p)
	}

	return persons, nil
}

// ============ FACES ============

// CreateFace добавляет новое лицо в базу
func (r *Repository) CreateFace(face *models.Face) error {
	_, err := r.db.Exec(`
		INSERT INTO faces (
			person_id, original_image, annotated_image,
			face_x, face_y, face_width, face_height,
			embedding, confidence
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, face.PersonID, face.OriginalImage, face.AnnotatedImage,
		face.FaceX, face.FaceY, face.FaceWidth, face.FaceHeight,
		face.Embedding, face.Confidence)

	return err
}

// ============ STATS ============

// GetStats возвращает общую статистику
func (r *Repository) GetStats() (*models.Stats, error) {
	var stats models.Stats

	err := r.db.Get(&stats.TotalPersons, "SELECT COUNT(*) FROM persons")
	if err != nil {
		return nil, err
	}

	err = r.db.Get(&stats.TotalFaces, "SELECT COUNT(*) FROM faces")
	if err != nil {
		return nil, err
	}

	err = r.db.Get(&stats.TotalTasks, "SELECT COUNT(*) FROM tasks")
	if err != nil {
		return nil, err
	}

	return &stats, nil
}
