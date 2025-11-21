package websocket

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

// Message типы сообщений для WebSocket
type MessageType string

const (
	MessageTypeTaskUpdate   MessageType = "task_update"
	MessageTypeTaskProgress MessageType = "task_progress"
	MessageTypeTaskComplete MessageType = "task_complete"
	MessageTypeTaskFailed   MessageType = "task_failed"
	MessageTypeStatsUpdate  MessageType = "stats_update"
)

// Message структура WebSocket сообщения
type Message struct {
	Type    MessageType `json:"type"`
	TaskID  string      `json:"task_id,omitempty"`
	Payload interface{} `json:"payload"`
}

// Client представляет WebSocket клиента
type Client struct {
	ID     string
	Conn   *websocket.Conn
	Send   chan Message
	TaskID string // ID задачи, которую отслеживает клиент
}

// Manager управляет WebSocket соединениями
type Manager struct {
	clients    map[string]*Client
	register   chan *Client
	unregister chan *Client
	broadcast  chan Message
	mu         sync.RWMutex
}

// NewManager создает новый WebSocket manager
func NewManager() *Manager {
	return &Manager{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan Message, 256),
	}
}

// Run запускает менеджер (должен работать в отдельной горутине)
func (m *Manager) Run() {
	for {
		select {
		case client := <-m.register:
			m.mu.Lock()
			m.clients[client.ID] = client
			m.mu.Unlock()
			log.Printf("WebSocket: клиент %s подключен (задача: %s)", client.ID, client.TaskID)

		case client := <-m.unregister:
			m.mu.Lock()
			if _, ok := m.clients[client.ID]; ok {
				delete(m.clients, client.ID)
				close(client.Send)
				log.Printf("WebSocket: клиент %s отключен", client.ID)
			}
			m.mu.Unlock()

		case message := <-m.broadcast:
			m.mu.RLock()
			for _, client := range m.clients {
				// Если сообщение для конкретной задачи - отправляем только подписанным клиентам
				if message.TaskID != "" && client.TaskID != message.TaskID {
					continue
				}

				select {
				case client.Send <- message:
				default:
					// Если канал переполнен - отключаем клиента
					close(client.Send)
					delete(m.clients, client.ID)
				}
			}
			m.mu.RUnlock()
		}
	}
}

// RegisterClient регистрирует нового клиента
func (m *Manager) RegisterClient(client *Client) {
	m.register <- client
}

// UnregisterClient отключает клиента
func (m *Manager) UnregisterClient(client *Client) {
	m.unregister <- client
}

// Broadcast отправляет сообщение всем клиентам
func (m *Manager) Broadcast(message Message) {
	m.broadcast <- message
}

// BroadcastTaskUpdate отправляет обновление по задаче
func (m *Manager) BroadcastTaskUpdate(taskID, status string, payload interface{}) {
	m.Broadcast(Message{
		Type:   MessageTypeTaskUpdate,
		TaskID: taskID,
		Payload: map[string]interface{}{
			"status": status,
			"data":   payload,
		},
	})
}

// BroadcastTaskProgress отправляет прогресс обработки
func (m *Manager) BroadcastTaskProgress(taskID string, current, total int, stage string) {
	m.Broadcast(Message{
		Type:   MessageTypeTaskProgress,
		TaskID: taskID,
		Payload: map[string]interface{}{
			"current": current,
			"total":   total,
			"stage":   stage,
			"percent": float64(current) / float64(total) * 100,
		},
	})
}

// BroadcastStatsUpdate отправляет обновление статистики
func (m *Manager) BroadcastStatsUpdate(stats interface{}) {
	m.Broadcast(Message{
		Type:    MessageTypeStatsUpdate,
		Payload: stats,
	})
}

// ReadPump читает сообщения от клиента
func (c *Client) ReadPump(manager *Manager) {
	defer func() {
		manager.UnregisterClient(c)
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Обрабатываем входящие сообщения от клиента (если нужно)
		log.Printf("Received from client %s: %s", c.ID, string(message))
	}
}

// WritePump отправляет сообщения клиенту
func (c *Client) WritePump() {
	defer func() {
		c.Conn.Close()
	}()

	for message := range c.Send {
		w, err := c.Conn.NextWriter(websocket.TextMessage)
		if err != nil {
			return
		}

		// Сериализуем сообщение в JSON
		data, err := json.Marshal(message)
		if err != nil {
			log.Printf("Error marshaling message: %v", err)
			continue
		}

		w.Write(data)

		if err := w.Close(); err != nil {
			return
		}
	}
}
