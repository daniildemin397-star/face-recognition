package websocket

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Разрешаем все origins (в продакшене нужно ограничить)
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Handler обрабатывает WebSocket подключения
type Handler struct {
	manager *Manager
}

// NewHandler создает новый WebSocket handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

// HandleWebSocket обрабатывает WebSocket подключение
func (h *Handler) HandleWebSocket(c *gin.Context) {
	// Получаем taskID из query параметра
	taskID := c.Query("task_id")

	// Апгрейдим HTTP соединение до WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade to WebSocket: %v", err)
		return
	}

	// Создаем клиента
	client := &Client{
		ID:     uuid.New().String(),
		Conn:   conn,
		Send:   make(chan Message, 256),
		TaskID: taskID,
	}

	// Регистрируем клиента
	h.manager.RegisterClient(client)

	// Запускаем горутины для чтения и записи
	go client.WritePump()
	go client.ReadPump(h.manager)
}

// GetManager возвращает менеджер (для использования в других handlers)
func (h *Handler) GetManager() *Manager {
	return h.manager
}
