package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"goapp/db"
	"goapp/middleware"
)

type registerInput struct {
	Username string `json:"username" binding:"required,min=3"`
	Password string `json:"password" binding:"required,min=6"`
}

type loginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// PageLogin / PageRegister rendent simplement les templates.
func PageLogin(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{})
}

func PageRegister(c *gin.Context) {
	c.HTML(http.StatusOK, "register.html", gin.H{})
}

// Register crée un compte utilisateur.
func Register(c *gin.Context) {
	var in registerInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur interne"})
		return
	}

	res, err := db.DB.Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, in.Username, string(hash))
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "nom d'utilisateur déjà pris"})
		return
	}
	userID, _ := res.LastInsertId()

	if err := middleware.CreateSession(c, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session impossible"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "redirect": "/dashboard"})
}

// Login vérifie les identifiants et crée une session.
func Login(c *gin.Context) {
	var in loginInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var userID int64
	var hash string
	row := db.DB.QueryRow(`SELECT id, password_hash FROM users WHERE username = ?`, in.Username)
	if err := row.Scan(&userID, &hash); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "identifiants invalides"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erreur interne"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "identifiants invalides"})
		return
	}

	if err := middleware.CreateSession(c, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "session impossible"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "redirect": "/dashboard"})
}

// Logout supprime la session courante.
func Logout(c *gin.Context) {
	middleware.DestroySession(c)
	c.Redirect(http.StatusFound, "/login")
}
