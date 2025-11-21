package main

import (
	"face-recognition/internal/api/handlers"
	"face-recognition/internal/api/middleware"
	"face-recognition/internal/api/websocket"
	"face-recognition/internal/config"
	"face-recognition/internal/repository"
	"face-recognition/internal/service/cache"
	"face-recognition/internal/service/storage"
	"face-recognition/pkg/python_client"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	// ASCII баннер
	printBanner()

	// Загружаем конфигурацию
	cfg := config.Load()
	log.Println("✅ Конфигурация загружена")

	// Инициализируем базу данных
	db, err := initDatabase(cfg.Database.GetDSN())
	if err != nil {
		log.Fatalf("❌ Ошибка подключения к БД: %v\n", err)
	}
	defer db.Close()
	log.Println("✅ База данных подключена")

	// Инициализируем Redis кэш
	var cacheService *cache.Service
	cacheService, err = cache.NewService(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		log.Printf("⚠️  Redis недоступен (работаем без кэша): %v\n", err)
		cacheService = nil
	} else {
		defer cacheService.Close()
		log.Println("✅ Redis кэш подключен")
	}

	// Инициализируем репозиторий
	repo := repository.NewRepository(db)

	// Инициализируем storage service
	storageService, err := storage.NewService(cfg.Storage.UploadsDir, cfg.Storage.ResultsDir)
	if err != nil {
		log.Fatalf("❌ Ошибка инициализации storage: %v\n", err)
	}
	log.Println("✅ Storage сервис инициализирован")

	// Инициализируем Python client
	pythonClient := python_client.NewClient(cfg.Python.BaseURL)

	// Проверяем доступность Python сервера
	if err := pythonClient.HealthCheck(); err != nil {
		log.Printf("⚠️  Предупреждение: Python сервер недоступен: %v\n", err)
		log.Println("💡 Запусти: cd python && python process.py")
	} else {
		log.Println("✅ Python сервер доступен")
	}

	// Инициализируем WebSocket manager
	wsManager := websocket.NewManager()
	go wsManager.Run() // Запускаем в отдельной горутине
	log.Println("✅ WebSocket manager запущен")

	// Инициализируем handlers (без face detector - всё делает Python)
	handler := handlers.NewHandler(repo, storageService, pythonClient, cacheService, wsManager)

	// Создаем роутер
	router := setupRouter(handler, wsManager, cfg)

	// Запускаем сервер
	addr := fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)
	log.Println("🎉 Сервер успешно запущен!")
	log.Printf("🌐 Веб-интерфейс: http://localhost:%s\n", cfg.Server.Port)
	log.Printf("📡 API: http://localhost:%s/api\n", cfg.Server.Port)
	log.Printf("🔌 WebSocket: ws://localhost:%s/ws\n", cfg.Server.Port)
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if err := router.Run(addr); err != nil {
		log.Fatalf("❌ Ошибка запуска сервера: %v\n", err)
	}
}

// initDatabase инициализирует подключение к базе данных
func initDatabase(dsn string) (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, err
	}

	// Проверяем подключение
	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Настраиваем connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return db, nil
}

// setupRouter настраивает роутер с middleware и endpoints
func setupRouter(handler *handlers.Handler, wsManager *websocket.Manager, cfg *config.Config) *gin.Engine {
	// Режим production для меньшего логирования
	// gin.SetMode(gin.ReleaseMode)

	router := gin.Default()

	// Middleware
	router.Use(middleware.CORS())
	router.Use(middleware.Recovery())

	// Статические файлы
	router.Static("/static", "./web/static")
	router.Static("/uploads", cfg.Storage.UploadsDir)
	router.StaticFile("/", "./web/static/index.html")

	// WebSocket endpoint
	wsHandler := websocket.NewHandler(wsManager)
	router.GET("/ws", wsHandler.HandleWebSocket)

	// API группа
	api := router.Group("/api")
	{
		// Загрузка и обработка
		api.POST("/upload", handler.HandleUpload)
		api.GET("/task/:id", handler.HandleTaskStatus)

		// Работа с людьми
		api.GET("/persons", handler.HandleGetPersons)
		api.GET("/persons/:id", handler.HandleGetPerson)
		api.PUT("/persons/:id", handler.HandleUpdatePerson)
		api.DELETE("/persons/:id", handler.HandleDeletePerson)

		// Поиск
		api.GET("/search", handler.HandleSearch)

		// Статистика
		api.GET("/stats", handler.HandleGetStats)
	}

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "face-recognition-api",
			"version": "2.0.0",
		})
	})

	return router
}

// printBanner печатает красивый баннер при старте
func printBanner() {
	banner := `
╔═══════════════════════════════════════════════════════╗
║                                                       ║
║   🎭  FACE RECOGNITION SYSTEM                        ║
║                                                       ║
║   Интеллектуальная система распознавания лиц         ║
║   с кластеризацией и идентификацией                  ║
║                                                       ║
║   Версия: 2.0.0                                      ║
║   Автор: Hackathon Team                              ║
║                                                       ║
╚═══════════════════════════════════════════════════════╝
`
	fmt.Println(banner)
	log.Println("🚀 Инициализация сервисов...")
}
