package main

import (
	"log"

	"goravel/app/models"
	ws "goravel/app/websockets"
	"goravel/bootstrap"
	"goravel/routes"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/gofiber/template/html/v3"
	"github.com/goravel/framework/facades"
)

// ensureAdminUser crea un usuario administrador si no existe en la base de datos.
func ensureAdminUser() {
	adminEmail := "admin@example.com"
	adminPassword := "Admin123!"
	adminName := "Administrador"
	adminRole := "admin"

	log.Println("🔍 Verificando existencia de usuario administrador...")

	var user models.User
	err := facades.Orm().Query().Where("email = ?", adminEmail).First(&user)

	if err == nil && user.ID > 0 {
		log.Printf("✅ Usuario administrador ya existe (ID: %d, Email: %s)", user.ID, user.Email)
		return
	}

	log.Printf("⚠️  Usuario administrador no encontrado. Creando...")

	hashed, err := facades.Hash().Make(adminPassword)
	if err != nil {
		log.Printf("❌ Error al hashear contraseña del admin: %v", err)
		return
	}

	admin := models.User{
		Name:     adminName,
		Email:    adminEmail,
		Password: hashed,
		Role:     adminRole,
		Status:   true,
	}

	if err := facades.Orm().Query().Create(&admin); err != nil {
		log.Printf("❌ Error al crear usuario administrador: %v", err)
		return
	}

	log.Printf("✅ Usuario administrador creado con éxito (ID: %d, Email: %s)", admin.ID, admin.Email)
}
func main() {
	_ = bootstrap.Boot()
	ensureAdminUser()
	engine := html.New("./app/views", ".html")
	engine.AddFunc("add", func(a, b int) int { return a + b })
	engine.AddFunc("sub", func(a, b int) int { return a - b })
	engine.Reload(true) // recarga en cada request
	engine.Debug(true)

	app := fiber.New(fiber.Config{Views: engine})
	app.Use(logger.New())
	app.Use(session.New())
	app.Use("/*", static.New("./public", static.Config{
		Browse: false,
	}))
	// WebSocket
	hub := ws.NewHub()
	go hub.Run()
	ws.RegisterRoutes(app, hub)

	// Rutas HTTP
	routes.Web(app)

	log.Println("🚀 ALCON escuchando en :3000")
	log.Fatal(app.Listen(":3000"))
}
