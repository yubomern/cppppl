package handlers

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"goapp/utils"
)

type telnetSession struct {
	conn   net.Conn
	name   string
	prompt string
	logger *utils.InterfaceLogger
	mu     sync.Mutex
}

var (
	telnetSessions   = map[string]*telnetSession{}
	telnetSessionsMu sync.Mutex
)

type telnetConnectInput struct {
	Host   string `json:"host" binding:"required"`
	Port   int    `json:"port"`
	Prompt string `json:"prompt"`
}

// ConnectTelnet ouvre une connexion TCP brute vers host:port (telnet).
// Remarque: on n'implémente pas la négociation IAC complète du protocole
// telnet (souvent inutile face à des équipements en mode texte simple),
// on envoie/lit du texte brut sur le socket.
func ConnectTelnet(c *gin.Context) {
	var in telnetConnectInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if in.Port == 0 {
		in.Port = 23
	}
	if in.Prompt == "" {
		in.Prompt = "TT>"
	}

	addr := fmt.Sprintf("%s:%d", in.Host, in.Port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "connexion impossible: " + err.Error()})
		return
	}

	key := addr
	logger, err := utils.NewInterfaceLogger("telnet_" + sanitizeName(strings.ReplaceAll(addr, ":", "_")))
	if err != nil {
		_ = conn.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "log impossible: " + err.Error()})
		return
	}

	sess := &telnetSession{conn: conn, name: key, prompt: in.Prompt, logger: logger}

	telnetSessionsMu.Lock()
	if old, ok := telnetSessions[key]; ok {
		_ = old.conn.Close()
		old.logger.Close()
	}
	telnetSessions[key] = sess
	telnetSessionsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"ok": true, "session": key, "prompt": in.Prompt, "log_file": logger.Path()})
}

type telnetCommandInput struct {
	Session string `json:"session" binding:"required"` // "host:port" renvoyé par ConnectTelnet
	Command string `json:"command" binding:"required"`
}

// SendTelnetCommand envoie une commande et lit la réponse jusqu'au prompt choisi.
func SendTelnetCommand(c *gin.Context) {
	var in telnetCommandInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	telnetSessionsMu.Lock()
	sess, ok := telnetSessions[in.Session]
	telnetSessionsMu.Unlock()
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session telnet inconnue"})
		return
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	toSend := in.Command
	if !strings.HasSuffix(toSend, "\r\n") {
		toSend += "\r\n"
	}
	sess.logger.Log("TX", in.Command)

	if _, err := sess.conn.Write([]byte(toSend)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "écriture impossible: " + err.Error()})
		return
	}

	_ = sess.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var sb strings.Builder
	buf := make([]byte, 512)
	for {
		n, err := sess.conn.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
			if strings.Contains(sb.String(), sess.prompt) {
				break
			}
		}
		if err != nil {
			break
		}
	}

	response := sb.String()
	sess.logger.Log("RX", response)
	c.JSON(http.StatusOK, gin.H{"response": response})
}

// DisconnectTelnet ferme une session telnet.
func DisconnectTelnet(c *gin.Context) {
	key := c.Param("session")
	telnetSessionsMu.Lock()
	sess, ok := telnetSessions[key]
	if ok {
		delete(telnetSessions, key)
	}
	telnetSessionsMu.Unlock()

	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session inconnue"})
		return
	}
	_ = sess.conn.Close()
	sess.logger.Close()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
