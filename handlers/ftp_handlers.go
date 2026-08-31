package handlers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jlaffaye/ftp"

	"goapp/utils"
)

type ftpSession struct {
	conn   *ftp.ServerConn
	name   string
	logger *utils.InterfaceLogger
	mu     sync.Mutex
}

var (
	ftpSessions   = map[string]*ftpSession{}
	ftpSessionsMu sync.Mutex
)

type ftpConnectInput struct {
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// ConnectFTP ouvre une connexion FTP et s'authentifie.
func ConnectFTP(c *gin.Context) {
	var in ftpConnectInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if in.Port == 0 {
		in.Port = 21
	}
	if in.Username == "" {
		in.Username = "anonymous"
	}

	addr := fmt.Sprintf("%s:%d", in.Host, in.Port)
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(5*time.Second))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "connexion impossible: " + err.Error()})
		return
	}
	if err := conn.Login(in.Username, in.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentification impossible: " + err.Error()})
		return
	}

	logger, err := utils.NewInterfaceLogger("ftp_" + sanitizeName(strings.ReplaceAll(addr, ":", "_")))
	if err != nil {
		_ = conn.Quit()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "log impossible: " + err.Error()})
		return
	}
	logger.Log("TX", fmt.Sprintf("LOGIN %s", in.Username))

	sess := &ftpSession{conn: conn, name: addr, logger: logger}
	ftpSessionsMu.Lock()
	if old, ok := ftpSessions[addr]; ok {
		_ = old.conn.Quit()
		old.logger.Close()
	}
	ftpSessions[addr] = sess
	ftpSessionsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{"ok": true, "session": addr, "log_file": logger.Path()})
}

// ListFTP liste le contenu d'un répertoire distant.
func ListFTP(c *gin.Context) {
	session := c.Query("session")
	path := c.DefaultQuery("path", ".")

	ftpSessionsMu.Lock()
	sess, ok := ftpSessions[session]
	ftpSessionsMu.Unlock()
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session ftp inconnue"})
		return
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	entries, err := sess.conn.List(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	sess.logger.Log("TX", "LIST "+path)

	var out []gin.H
	for _, e := range entries {
		out = append(out, gin.H{
			"name": e.Name,
			"type": e.Type.String(),
			"size": e.Size,
			"time": e.Time.Format("2006-01-02 15:04:05"),
		})
	}
	sess.logger.Log("RX", fmt.Sprintf("%d entrées", len(out)))
	c.JSON(http.StatusOK, gin.H{"entries": out})
}

// UploadFTP envoie un fichier reçu (multipart) vers le serveur FTP distant.
func UploadFTP(c *gin.Context) {
	session := c.PostForm("session")
	remotePath := c.PostForm("remote_path")

	ftpSessionsMu.Lock()
	sess, ok := ftpSessions[session]
	ftpSessionsMu.Unlock()
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session ftp inconnue"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fichier manquant"})
		return
	}
	defer file.Close()

	if remotePath == "" {
		remotePath = header.Filename
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	if err := sess.conn.Stor(remotePath, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	sess.logger.Log("TX", "STOR "+remotePath)
	c.JSON(http.StatusOK, gin.H{"ok": true, "remote_path": remotePath})
}

// DownloadFTP télécharge un fichier distant et le renvoie au client.
func DownloadFTP(c *gin.Context) {
	session := c.Query("session")
	remotePath := c.Query("path")

	ftpSessionsMu.Lock()
	sess, ok := ftpSessions[session]
	ftpSessionsMu.Unlock()
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session ftp inconnue"})
		return
	}

	sess.mu.Lock()
	defer sess.mu.Unlock()

	r, err := sess.conn.Retr(remotePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer r.Close()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, r); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	sess.logger.Log("TX", "RETR "+remotePath)

	c.Header("Content-Disposition", "attachment; filename="+remotePath)
	c.Data(http.StatusOK, "application/octet-stream", buf.Bytes())
}

// DisconnectFTP ferme la session FTP.
func DisconnectFTP(c *gin.Context) {
	session := c.Query("session")
	ftpSessionsMu.Lock()
	sess, ok := ftpSessions[session]
	if ok {
		delete(ftpSessions, session)
	}
	ftpSessionsMu.Unlock()

	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session inconnue"})
		return
	}
	_ = sess.conn.Quit()
	sess.logger.Close()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
