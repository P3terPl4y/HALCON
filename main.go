package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"goravel/app/models"
	ws "goravel/app/websockets"
	"goravel/bootstrap"
	"goravel/routes"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/middleware/csrf"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/fiber/v3/middleware/static"
	"github.com/gofiber/storage/redis/v3"
	"github.com/gofiber/template/html/v3"
	"github.com/goravel/framework/facades"
)

// ─────────────────────────────────────────────────────────────
// Helpers de entorno
// ─────────────────────────────────────────────────────────────

var isProd = os.Getenv("APP_ENV") == "production"

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// ensureAdminUser crea un usuario administrador si no existe.
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

	log.Println("⚠️  Usuario administrador no encontrado. Creando...")

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

// ─────────────────────────────────────────────────────────────
// main
// ─────────────────────────────────────────────────────────────

func main() {
	// ── 1. Bootstrap de Goravel ──
	_ = bootstrap.Boot()
	ensureAdminUser()

	// ── 2. Store de Redis para sesiones ──
	redisStore := redis.New(redis.Config{
		Host:     env("REDIS_HOST", "127.0.0.1"),
		Port:     envInt("REDIS_PORT", 6379),
		Username: env("REDIS_USERNAME", ""),
		Password: env("REDIS_PASSWORD", ""),
		Database: envInt("REDIS_DB", 0),
		PoolSize: envInt("REDIS_POOL_SIZE", 10),
	})

	// ── 3. Motor de plantillas ──
	log.Println("🖼️  Configurando motor de plantillas HTML...")
	engine := html.New("./app/views", ".html")
	engine.Reload(!isProd)
	engine.Debug(!isProd)
	engine.AddFunc("add", func(a, b int) int { return a + b })
	engine.AddFunc("sub", func(a, b int) int { return a - b })
	engine.AddFunc("deref", func(p *uint) uint {
		if p == nil {
			return 0
		}
		return *p
	})

	// ── 4. App Fiber (una sola, con todo) ──
	app := fiber.New(fiber.Config{
		Views:       engine,
		TrustProxy:  true,
		ProxyHeader: fiber.HeaderXForwardedFor,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Loopback: true,
		},
	})

	// ── 5. Middlewares base ──
	log.Println("🔒 Configurando middlewares...")
	app.Use(logger.New())
	app.Use(recover.New())

	// ── 6. Helmet (con ws: y wss: en connect-src) ──
	app.Use(helmet.New(helmet.Config{
		ContentSecurityPolicy: "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net https://unpkg.com; " +
			"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net https://unpkg.com; " +
			"img-src 'self' data: blob: https://*.tile.openstreetmap.org https://tile.openstreetmap.org https://unpkg.com; " +
			"font-src 'self' https://cdn.jsdelivr.net; " +
			"connect-src 'self' ws: wss: https://nominatim.openstreetmap.org; " +
			"frame-ancestors 'none';",
		HSTSMaxAge:                31536000,
		HSTSPreloadEnabled:        true,
		XFrameOptions:             "DENY",
		XSSProtection:             "1; mode=block",
		ContentTypeNosniff:        "nosniff",
		ReferrerPolicy:            "strict-origin-when-cross-origin",
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginEmbedderPolicy: "",
		CrossOriginResourcePolicy: "",
	}))

	// ── 7. Quitar cabeceras que rompen los tiles de OSM ──
	app.Use(func(c fiber.Ctx) error {
		c.Response().Header.Del("Cross-Origin-Embedder-Policy")
		c.Response().Header.Del("Cross-Origin-Resource-Policy")
		return c.Next()
	})

	// ── 8. Sesiones con Redis ──
	log.Println("🍪 Configurando middleware de sesiones...")
	sessionCookieName := "session_id"
	if isProd {
		sessionCookieName = "__Host-session"
	}

	sessionMiddleware, sessionStore := session.NewWithStore(session.Config{
		Storage:           redisStore,
		Extractor:         extractors.FromCookie(sessionCookieName),
		CookieSecure:      isProd,
		CookieHTTPOnly:    true,
		CookieSameSite:    "Lax",
		CookieSessionOnly: true,
		IdleTimeout:       30 * time.Minute,
		AbsoluteTimeout:   24 * time.Hour,
	})
	app.Use(sessionMiddleware)
	log.Println("✅ Middleware de sesiones configurado")

	// ── 9. CSRF ──
	csrfCookieName := "csrf_"
	if isProd {
		csrfCookieName = "__Host-csrf_"
	}

	app.Use(csrf.New(csrf.Config{
		CookieName:     csrfCookieName,
		CookieSecure:   isProd,
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
		Extractor:      extractors.FromForm("_csrf"),
		IdleTimeout:    30 * time.Minute,
		Session:        sessionStore,
		TrustedOrigins: []string{
			"https://mariana-flagless-inaudibly.ngrok-free.dev","http://localhost:3300",
		},
	}))

	// ── 10. Archivos estáticos (por prefijo, no catch-all) ──
	// Nota: app.Static() fue eliminado en Fiber v3. Ahora es middleware.
	app.Use("/css",     static.New("./public/css"))
	app.Use("/js",      static.New("./public/js"))
	app.Use("/img",     static.New("./public/img"))
	app.Use("/fonts",   static.New("./public/fonts"))
	app.Use("/leaflet", static.New("./public/leaflet"))

	// ── 11. WebSockets ──
	hub := ws.NewHub()
	go hub.Run()
	ws.RegisterRoutes(app, hub)

	// ── 12. Rutas HTTP ──
	routes.Web(app)

	// ── 13. Arranque ──
	log.Println("🚀 ALCON escuchando en :3300")
	log.Fatal(app.Listen(":3300"))
}
