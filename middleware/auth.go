package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"goapp/db"
)

const cookieName = "session_token"

// GenerateToken crée un jeton aléatoire de session.
func GenerateToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// CreateSession insère une session en base et pose le cookie.
func CreateSession(c *gin.Context, userID int64) error {
	token := GenerateToken()
	_, err := db.DB.Exec(`INSERT INTO sessions (token, user_id) VALUES (?, ?)`, token, userID)
	if err != nil {
		return err
	}
	c.SetCookie(cookieName, token, 3600*24*7, "/", "", false, true)
	return nil
}

// DestroySession supprime la session courante.
func DestroySession(c *gin.Context) {
	token, err := c.Cookie(cookieName)
	if err == nil {
		_, _ = db.DB.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	}
	c.SetCookie(cookieName, "", -1, "/", "", false, true)
}

// CurrentUserID renvoie l'id utilisateur associé au cookie de session, ou 0.
func CurrentUserID(c *gin.Context) int64 {
	token, err := c.Cookie(cookieName)
	if err != nil || token == "" {
		return 0
	}
	var userID int64
	row := db.DB.QueryRow(`SELECT user_id FROM sessions WHERE token = ?`, token)
	if err := row.Scan(&userID); err != nil {
		return 0
	}
	return userID
}

// RequireAuth protège les routes qui nécessitent une session valide.
// Renvoie un 401 JSON pour les appels AJAX (/api/...) ou redirige vers /login sinon.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := CurrentUserID(c)
		if uid == 0 {
			if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "non authentifié"})
				return
			}
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Set("user_id", uid)
		c.Next()
	}
}
