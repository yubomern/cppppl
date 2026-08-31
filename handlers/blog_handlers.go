package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"goapp/db"
	"goapp/models"
)

// PageBlog rend la page listant les articles.
func PageBlog(c *gin.Context) {
	c.HTML(http.StatusOK, "blog.html", gin.H{})
}

// PageBlogEdit rend la page d'édition (CKEditor) d'un article, nouveau ou existant.
func PageBlogEdit(c *gin.Context) {
	c.HTML(http.StatusOK, "blog_edit.html", gin.H{"PostID": c.Param("id")})
}

// ListPosts renvoie tous les articles (JSON, pour AJAX).
func ListPosts(c *gin.Context) {
	rows, err := db.DB.Query(`
		SELECT p.id, p.title, p.body, p.image_path, p.created_at, p.updated_at, u.username
		FROM posts p JOIN users u ON u.id = p.author_id
		ORDER BY p.created_at DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var posts []models.Post
	for rows.Next() {
		var p models.Post
		if err := rows.Scan(&p.ID, &p.Title, &p.Body, &p.ImagePath, &p.CreatedAt, &p.UpdatedAt, &p.Author); err != nil {
			continue
		}
		posts = append(posts, p)
	}
	c.JSON(http.StatusOK, gin.H{"posts": posts})
}

// GetPost renvoie un article précis.
func GetPost(c *gin.Context) {
	id := c.Param("id")
	var p models.Post
	row := db.DB.QueryRow(`
		SELECT p.id, p.title, p.body, p.image_path, p.created_at, p.updated_at, u.username
		FROM posts p JOIN users u ON u.id = p.author_id WHERE p.id = ?`, id)
	if err := row.Scan(&p.ID, &p.Title, &p.Body, &p.ImagePath, &p.CreatedAt, &p.UpdatedAt, &p.Author); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "article introuvable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"post": p})
}

type postInput struct {
	Title string `json:"title" binding:"required"`
	Body  string `json:"body" binding:"required"` // HTML produit par CKEditor
}

// CreatePost crée un nouvel article (title + body venant de CKEditor).
func CreatePost(c *gin.Context) {
	var in postInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	authorID := c.GetInt64("user_id")

	res, err := db.DB.Exec(`INSERT INTO posts (title, body, author_id) VALUES (?, ?, ?)`, in.Title, in.Body, authorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	id, _ := res.LastInsertId()
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": id})
}

// UpdatePost met à jour un article existant.
func UpdatePost(c *gin.Context) {
	id := c.Param("id")
	var in postInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := db.DB.Exec(`UPDATE posts SET title = ?, body = ?, updated_at = ? WHERE id = ?`,
		in.Title, in.Body, time.Now(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// DeletePost supprime un article.
func DeletePost(c *gin.Context) {
	id := c.Param("id")
	if _, err := db.DB.Exec(`DELETE FROM posts WHERE id = ?`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// UploadBlogImage reçoit une image envoyée par CKEditor (endpoint SimpleUploadAdapter)
// et répond au format attendu par CKEditor: {"url": "..."}
func UploadBlogImage(c *gin.Context) {
	file, header, err := c.Request.FormFile("upload")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "fichier manquant"}})
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	newName := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	dst := filepath.Join("static", "uploads", newName)

	if err := c.SaveUploadedFile(header, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	authorID := c.GetInt64("user_id")
	_, _ = db.DB.Exec(`INSERT INTO files (filename, path, size, uploader_id) VALUES (?, ?, ?, ?)`,
		header.Filename, "/static/uploads/"+newName, header.Size, authorID)

	// Format de réponse attendu par le SimpleUploadAdapter de CKEditor
	c.JSON(http.StatusOK, gin.H{"url": "/static/uploads/" + newName})
}
