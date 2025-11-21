package config

import (
	"fmt"
	"os"
)

// Config содержит всю конфигурацию приложения
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Storage  StorageConfig
	Python   PythonConfig
	Redis    RedisConfig
}

// ServerConfig - настройки HTTP сервера
type ServerConfig struct {
	Port string
	Host string
}

// DatabaseConfig - настройки базы данных
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// StorageConfig - настройки хранилища файлов
type StorageConfig struct {
	UploadsDir string
	ResultsDir string
}

// PythonConfig - настройки Python сервера
type PythonConfig struct {
	BaseURL string
}

// RedisConfig - настройки Redis
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// Load загружает конфигурацию из переменных окружения
// с fallback на значения по умолчанию
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "faceuser"),
			Password: getEnv("DB_PASSWORD", "facepass"),
			DBName:   getEnv("DB_NAME", "facedb"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Storage: StorageConfig{
			UploadsDir: getEnv("UPLOADS_DIR", "uploads"),
			ResultsDir: getEnv("RESULTS_DIR", "results"),
		},
		Python: PythonConfig{
			BaseURL: getEnv("PYTHON_BASE_URL", "http://localhost:5000"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
	}
}

// GetDSN возвращает строку подключения к PostgreSQL
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

// getEnv получает переменную окружения или возвращает значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt получает целочисленную переменную окружения
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		fmt.Sscanf(value, "%d", &intValue)
		return intValue
	}
	return defaultValue
}
