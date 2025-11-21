package middleware

import (
	"github.com/gin-gonic/gin"
)

// CORS добавляет заголовки CORS к ответам
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		// Обработка preflight запросов
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// Logger логирует каждый запрос
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Можно добавить более детальное логирование
		// log.Printf("[%s] %s %s", c.Request.Method, c.Request.URL.Path, c.ClientIP())
		c.Next()
	}
}

// Recovery восстанавливает приложение после паники
func Recovery() gin.HandlerFunc {
	return gin.Recovery()
}
