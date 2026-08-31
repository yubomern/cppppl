package handlers

import (
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"goapp/db"
	"goapp/models"
)

// PageChat rend la page de chat.
func PageChat(c *gin.Context) {
	c.HTML(http.StatusOK, "chat.html", gin.H{})
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // à restreindre en production
}

type chatClient struct {
	conn *websocket.Conn
	send chan models.ChatMessage
}

var (
	chatClients   = map[*chatClient]bool{}
	chatClientsMu sync.Mutex
)

// ChatWS gère la connexion websocket d'un client au salon de chat.
func ChatWS(c *gin.Context) {
	username := c.Query("username")
	if username == "" {
		username = "anonyme"
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("chat: upgrade websocket échoué:", err)
		return
	}

	client := &chatClient{conn: conn, send: make(chan models.ChatMessage, 16)}
	chatClientsMu.Lock()
	chatClients[client] = true
	chatClientsMu.Unlock()

	go client.writePump()
	client.readPump(username)
}

func (cl *chatClient) readPump(username string) {
	defer func() {
		chatClientsMu.Lock()
		delete(chatClients, cl)
		chatClientsMu.Unlock()
		close(cl.send)
		cl.conn.Close()
	}()

	for {
		var incoming struct {
			Message string `json:"message"`
		}
		if err := cl.conn.ReadJSON(&incoming); err != nil {
			break
		}
		if incoming.Message == "" {
			continue
		}

		msg := models.ChatMessage{Username: username, Message: incoming.Message}
		res, err := db.DB.Exec(`INSERT INTO chat_messages (username, message) VALUES (?, ?)`, msg.Username, msg.Message)
		if err == nil {
			id, _ := res.LastInsertId()
			msg.ID = id
		}
		broadcastChat(msg)
	}
}

func (cl *chatClient) writePump() {
	for msg := range cl.send {
		if err := cl.conn.WriteJSON(msg); err != nil {
			break
		}
	}
}

func broadcastChat(msg models.ChatMessage) {
	chatClientsMu.Lock()
	defer chatClientsMu.Unlock()
	for cl := range chatClients {
		select {
		case cl.send <- msg:
		default:
			// client trop lent: on ignore ce message pour lui
		}
	}
}

// ChatHistory renvoie les N derniers messages du chat (AJAX, chargement initial).
func ChatHistory(c *gin.Context) {
	rows, err := db.DB.Query(`SELECT id, username, message, created_at FROM chat_messages ORDER BY id DESC LIMIT 50`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var msgs []models.ChatMessage
	for rows.Next() {
		var m models.ChatMessage
		if err := rows.Scan(&m.ID, &m.Username, &m.Message, &m.CreatedAt); err != nil {
			continue
		}
		msgs = append(msgs, m)
	}
	// on les remet dans l'ordre chronologique
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	c.JSON(http.StatusOK, gin.H{"messages": msgs})
}
