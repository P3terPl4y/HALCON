package routes

import (
	"goravel/app/http/controllers"
	"goravel/app/http/middlewares"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"time"
)

func Web(app *fiber.App) {
	// --- Ruta raíz ---
	app.Get("/", func(c fiber.Ctx) error {
		return c.Render("landing", fiber.Map{})
	})

	// --- Rutas públicas (sin auth) ---
	app.Use("/api", func(c fiber.Ctx) error { c.Set("Cache-Control", "no-store"); return c.Next() })
	loginLimit := limiter.New(limiter.Config{Max: 10, Expiration: time.Minute})
	app.Get("/api/session", controllers.SessionInfo)
	app.Post("/api/auth/login", loginLimit, controllers.APILogin)
	app.Post("/api/auth/logout", controllers.APILogout)
	authCtrl := controllers.NewAuthController()
	app.Get("/register", authCtrl.ShowRegister)
	app.Post("/register", limiter.New(limiter.Config{Max: 10, Expiration: time.Minute}), authCtrl.Register)
	app.Get("/login", authCtrl.ShowLogin)
	app.Post("/login", loginLimit, authCtrl.Login)
	app.Post("/logout", authCtrl.Logout)

	// --- Rutas protegidas (requieren sesión activa) ---
	usrCtrl := controllers.NewUserController()

	protected := app.Group("/", middlewares.AuthMiddleware())
	protected.Get("/profile", usrCtrl.Show)
	protected.Get("/api/tracking", usrCtrl.Tracking)
	protected.Post("/api/tracking/recipient", usrCtrl.ShareAPI)
	protected.Post("/profile/share", usrCtrl.Share)
	protected.Get("/profile/edit", usrCtrl.Edit)
	protected.Post("/profile/edit", usrCtrl.Update)
	protected.Get("/users", middlewares.ModeratorMiddleware(), usrCtrl.Index)

	// --- Panel de moderador (auth + rol) ---
	halconCtrl := controllers.NewHalconController()
	moderator := app.Group("/moderator",
		middlewares.AuthMiddleware(),
		middlewares.ModeratorMiddleware(),
	)
	moderator.Get("/halcon", func(c fiber.Ctx) error { return c.Redirect().To("/moderator/halcones") })
	moderator.Get("/halcones/:id/editor", halconCtrl.Editor)
	moderator.Get("/halcones", halconCtrl.Index)
	moderator.Post("/halcones", halconCtrl.Store)
	moderator.Post("/halcones/assign", halconCtrl.Assign)
	moderator.Delete("/halcones/:id", halconCtrl.Destroy)
	moderator.Post("/halcones/:id/delete", halconCtrl.Destroy)

	// --- Panel de Administrador ---
	admin := app.Group("/admin",
		middlewares.AuthMiddleware(),
		middlewares.AdminMiddleware(),
	)

	// Dashboard admin
	admin.Get("/", func(c fiber.Ctx) error {
		return c.Redirect().To("/admin/users")
	})

	// CRUD Usuarios
	adminUserCtrl := controllers.NewAdminUserController()
	admin.Get("/users", adminUserCtrl.Index)
	admin.Get("/users/create", adminUserCtrl.Create)
	admin.Post("/users", adminUserCtrl.Store)
	admin.Get("/users/:id/edit", adminUserCtrl.Edit)
	admin.Post("/users/:id", adminUserCtrl.Update)
	admin.Post("/users/:id/delete", adminUserCtrl.Destroy)

	// CRUD Halcones
	adminHalconCtrl := controllers.NewAdminHalconController()
	admin.Get("/halcones", adminHalconCtrl.Index)
	admin.Get("/halcones/create", adminHalconCtrl.Create)
	admin.Post("/halcones", adminHalconCtrl.Store)
	admin.Get("/halcones/:id", adminHalconCtrl.Show)
	admin.Get("/halcones/:id/edit", adminHalconCtrl.Edit)
	admin.Post("/halcones/:id", adminHalconCtrl.Update)
	admin.Post("/halcones/:id/delete", adminHalconCtrl.Destroy)
	admin.Get("/halcones/:id/assign", adminHalconCtrl.AssignForm)
	admin.Post("/halcones/:id/assign", adminHalconCtrl.AssignStore)
}
