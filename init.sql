-- Таблица для хранения информации о людях
CREATE TABLE IF NOT EXISTS persons (
                                       id SERIAL PRIMARY KEY,
                                       name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
    );

-- Таблица для хранения лиц (embeddings)
CREATE TABLE IF NOT EXISTS faces (
                                     id SERIAL PRIMARY KEY,
                                     person_id INTEGER REFERENCES persons(id) ON DELETE CASCADE,

    -- Пути к изображениям
    original_image VARCHAR(500) NOT NULL,
    annotated_image VARCHAR(500),

    -- Координаты лица на оригинальном фото
    face_x INTEGER NOT NULL DEFAULT 0,
    face_y INTEGER NOT NULL DEFAULT 0,
    face_width INTEGER NOT NULL DEFAULT 0,
    face_height INTEGER NOT NULL DEFAULT 0,

    -- ML данные
    embedding BYTEA,
    confidence FLOAT DEFAULT 0.0,

    detected_at TIMESTAMP DEFAULT NOW()
    );

-- Таблица для истории задач обработки
CREATE TABLE IF NOT EXISTS tasks (
                                     id VARCHAR(36) PRIMARY KEY,
    status VARCHAR(20) NOT NULL,
    total_images INTEGER DEFAULT 0,
    total_faces INTEGER DEFAULT 0,
    unique_persons INTEGER DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    completed_at TIMESTAMP
    );

-- Индексы для быстрого поиска
CREATE INDEX IF NOT EXISTS idx_faces_person_id ON faces(person_id);
CREATE INDEX IF NOT EXISTS idx_persons_name ON persons(name);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);

-- Функция для автоматического обновления updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
RETURN NEW;
END;
$$ language 'plpgsql';

-- Триггер для persons
DROP TRIGGER IF EXISTS update_persons_updated_at ON persons;
CREATE TRIGGER update_persons_updated_at BEFORE UPDATE ON persons
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();