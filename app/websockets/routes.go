package websocket

import (
	"log"

	"goravel/app/services"

	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
)

func RegisterRoutes(app *fiber.App, hub *Hub) {
	halconService := services.NewHalconService()

	// --- Halcones: upgrade sin auth de sesión (usan token) ---
	app.Use("/halcon", func(c fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// --- Dashboard: aquí sí hay fiber.Ctx, así que la sesión funciona ---
	app.Use("/dashboard", func(c fiber.Ctx) error {
		if !websocket.IsWebSocketUpgrade(c) {
			return fiber.ErrUpgradeRequired
		}

		sess := session.FromContext(c)
		if sess == nil {
			log.Println("❌ WS dashboard[middleware]: session.FromContext devolvió nil")
			return fiber.ErrUnauthorized
		}

		raw := sess.Get("user_id")
		if raw == nil {
			log.Println("❌ WS dashboard[middleware]: user_id no está en la sesión")
			return fiber.ErrUnauthorized
		}

		var uid uint
		switch v := raw.(type) {
		case uint:
			uid = v
		case int:
			uid = uint(v)
		case int64:
			uid = uint(v)
		case float64:
			uid = uint(v)
		default:
			log.Printf("❌ WS dashboard[middleware]: tipo no soportado %T", raw)
			return fiber.ErrUnauthorized
		}

		// ESTA línea es la que salva todo:
		// el wrapper websocket.New copia c.Locals() al *websocket.Conn
		c.Locals("user_id", uid)
		c.Locals("allowed", true)

		log.Printf("✅ WS dashboard[middleware]: sesión OK, user_id=%d", uid)
		return c.Next()
	})

	app.Get("/halcon/:id_user/:id_halcon/",
		websocket.New(HandleHalcon(hub, halconService)))

	app.Get("/dashboard/:id_user/",
		websocket.New(HandleDashboard(hub, halconService)))
}