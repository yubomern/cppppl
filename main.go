package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"goapp/db"
	"goapp/handlers"
	"goapp/middleware"
	"goapp/utils"
)

func main() {
	db.Init("app.db")
	utils.SetLogDir("logs")

	r := gin.Default()
	r.LoadHTMLGlob("templates/*.html")
	r.Static("/static", "./static")

	// --- Pages publiques -----------------------------------------------
	r.GET("/", func(c *gin.Context) { c.Redirect(302, "/login") })
	r.GET("/login", handlers.PageLogin)
	r.GET("/register", handlers.PageRegister)
	r.POST("/api/register", handlers.Register)
	r.POST("/api/login", handlers.Login)
	r.GET("/logout", handlers.Logout)

	// --- Pages protégées -------------------------------------------------
	auth := r.Group("/")
	auth.Use(middleware.RequireAuth())
	{
		auth.GET("/dashboard", func(c *gin.Context) { c.HTML(200, "dashboard.html", gin.H{}) })
		auth.GET("/blog", handlers.PageBlog)
		auth.GET("/blog/new", handlers.PageBlogEdit)
		auth.GET("/blog/:id/edit", handlers.PageBlogEdit)
		auth.GET("/files", handlers.PageFiles)
		auth.GET("/serial", func(c *gin.Context) { c.HTML(200, "serial.html", gin.H{}) })
		auth.GET("/network", func(c *gin.Context) { c.HTML(200, "network.html", gin.H{}) })
		auth.GET("/ftp", func(c *gin.Context) { c.HTML(200, "ftp.html", gin.H{}) })
		auth.GET("/telnet", func(c *gin.Context) { c.HTML(200, "telnet.html", gin.H{}) })
		auth.GET("/chat", handlers.PageChat)
	}

	// --- API JSON protégée ------------------------------------------------
	api := r.Group("/api")
	api.Use(middleware.RequireAuth())
	{
		// Blog
		api.GET("/posts", handlers.ListPosts)
		api.GET("/posts/:id", handlers.GetPost)
		api.POST("/posts", handlers.CreatePost)
		api.PUT("/posts/:id", handlers.UpdatePost)
		api.DELETE("/posts/:id", handlers.DeletePost)
		api.POST("/upload/image", handlers.UploadBlogImage) // endpoint CKEditor

		// Fichiers
		api.GET("/files", handlers.ListFiles)
		api.POST("/files/upload", handlers.UploadFile)
		api.DELETE("/files/:id", handlers.DeleteFile)

		// Réseau
		api.GET("/network/interfaces", handlers.ListInterfaces)
		api.GET("/network/ip-a", handlers.RawIPA)
		api.GET("/network/interface/:name", handlers.InterfaceDetail)
		api.GET("/network/wifi", handlers.WifiInfoHandler)

		// Port série
		api.GET("/serial/scan", handlers.ScanSerialPorts)
		api.GET("/serial/sessions", handlers.ListSerialSessions)
		api.POST("/serial/connect", handlers.ConnectSerial)
		api.POST("/serial/command", handlers.SendSerialCommand)
		api.POST("/serial/disconnect/:port", handlers.DisconnectSerial)

		// Telnet
		api.POST("/telnet/connect", handlers.ConnectTelnet)
		api.POST("/telnet/command", handlers.SendTelnetCommand)
		api.POST("/telnet/disconnect", handlers.DisconnectTelnet) // ?session=host:port

		// FTP
		api.POST("/ftp/connect", handlers.ConnectFTP)
		api.GET("/ftp/list", handlers.ListFTP)         // ?session=&path=
		api.POST("/ftp/upload", handlers.UploadFTP)    // multipart: session, remote_path, file
		api.GET("/ftp/download", handlers.DownloadFTP) // ?session=&path=
		api.POST("/ftp/disconnect", handlers.DisconnectFTP) // ?session=

		// Chat (historique ; le websocket lui-même est hors du groupe /api ci-dessous)
		api.GET("/chat/history", handlers.ChatHistory)
	}

	// Websocket du chat (accepte le cookie de session normalement, mais le
	// username est passé en query pour simplifier le JS ci-joint)
	r.GET("/ws/chat", middleware.RequireAuth(), handlers.ChatWS)

	log.Println("Serveur démarré sur http://localhost:8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal(err)
	}
}
