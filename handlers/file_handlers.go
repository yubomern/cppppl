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

// PageFiles rend la page de gestion de fichiers.
func PageFiles(c *gin.Context) {
	c.HTML(http.StatusOK, "files.html", gin.H{})
}

// ListFiles renvoie la liste des fichiers uploadés (base locale, distinct du FTP distant).
func ListFiles(c *gin.Context) {
	rows, err := db.DB.Query(`SELECT id, filename, path, size, uploader_id, created_at FROM files ORDER BY created_at DESC`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	var files []models.FileRecord
	for rows.Next() {
		var f models.FileRecord
		if err := rows.Scan(&f.ID, &f.Filename, &f.Path, &f.Size, &f.UploaderID, &f.CreatedAt); err != nil {
			continue
		}
		files = append(files, f)
	}
	c.JSON(http.StatusOK, gin.H{"files": files})
}

// UploadFile gère l'upload générique d'un fichier local (stocké sous static/uploads).
func UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fichier manquant"})
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	newName := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	dst := filepath.Join("static", "uploads", newName)

	if err := c.SaveUploadedFile(header, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	uploaderID := c.GetInt64("user_id")
	res, err := db.DB.Exec(`INSERT INTO files (filename, path, size, uploader_id) VALUES (?, ?, ?, ?)`,
		header.Filename, "/static/uploads/"+newName, header.Size, uploaderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	id, _ := res.LastInsertId()
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": id, "path": "/static/uploads/" + newName})
}

// DeleteFile supprime l'entrée d'un fichier (le fichier physique reste sur disque
// pour simplicité; à adapter si suppression physique nécessaire).
func DeleteFile(c *gin.Context) {
	id := c.Param("id")
	if _, err := db.DB.Exec(`DELETE FROM files WHERE id = ?`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
