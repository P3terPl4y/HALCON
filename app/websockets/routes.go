package websocket

import (
	"context"
	"github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
	"goravel/app/http/middlewares"
	"goravel/app/services"
	"net/url"
	"strings"
	"time"
)

func RegisterRoutes(app *fiber.App, hub *Hub, store *session.Store) {
	service := services.NewHalconService()
	authenticated := func(c fiber.Ctx) error {
		if !websocket.IsWebSocketUpgrade(c) {
			return fiber.ErrUpgradeRequired
		}
		if !validBrowserOrigin(c.Get("Origin"), c.Get("Host"), c.Scheme()) {
			return fiber.ErrForbidden
		}
		sess := session.FromContext(c)
		if sess == nil {
			return fiber.ErrUnauthorized
		}
		uid, ok := middlewares.SessionUserID(sess.Get("user_id"))
		if !ok || store == nil {
			return fiber.ErrUnauthorized
		}
		u, err := services.NewUserService().GetByID(uid)
		if err != nil || !u.Status {
			return fiber.ErrUnauthorized
		}
		c.Locals("user_id", uid)
		c.Locals("session_expires", time.Now().Add(30*time.Minute))
		if store != nil {
			sessionID := strings.Clone(sess.ID())
			c.Locals("session_valid", func() bool {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				saved, err := store.GetByID(ctx, sessionID)
				if err != nil {
					return false
				}
				defer saved.Release()
				storedID, ok := middlewares.SessionUserID(saved.Get("user_id"))
				return ok && storedID == uid
			})
		}
		return c.Next()
	}
	app.Get("/location", authenticated, websocket.New(HandlePersonal(hub, service)))
	app.Get("/dashboard/:id_user/", authenticated, websocket.New(HandleDashboard(hub, service)))
	app.Get("/halcon/:id_user/:id_halcon/", func(c fiber.Ctx) error {
		if !websocket.IsWebSocketUpgrade(c) {
			return fiber.ErrUpgradeRequired
		}
		return c.Next()
	}, websocket.New(HandleHalcon(hub, service)))
}

func validBrowserOrigin(raw, host, scheme string) bool {
	origin, err := url.Parse(raw)
	return err == nil && origin.Host == host && origin.Scheme == scheme && origin.User == nil && origin.Path == "" && origin.RawQuery == "" && origin.Fragment == "" && (scheme == "http" || scheme == "https")
}
