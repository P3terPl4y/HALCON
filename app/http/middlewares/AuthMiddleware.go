package middlewares

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
	"goravel/app/services"
	"log"
	"strings"
)

func AuthMiddleware() fiber.Handler {
	return func(ctx fiber.Ctx) error {
		ctx.Set("Cache-Control", "no-store")
		unauthenticated := func() error {
			if strings.HasPrefix(ctx.Path(), "/api/") {
				return ctx.Status(401).JSON(fiber.Map{"error": "Inicia sesión para continuar."})
			}
			return ctx.Redirect().To("/login")
		}
		sess := session.FromContext(ctx)
		if sess == nil {
			log.Printf("AuthMiddleware - Sesión nil en %s", ctx.Path())
			return unauthenticated()
		}

		userID := sess.Get("user_id")
		if userID == nil {
			log.Printf("AuthMiddleware - user_id no encontrado en sesión para %s", ctx.Path())
			return unauthenticated()
		}

		uid, ok := SessionUserID(userID)
		if !ok {
			return unauthenticated()
		}

		user, err := services.NewUserService().GetByID(uid)
		if err != nil || !user.Status {
			return fiber.ErrUnauthorized
		}
		ctx.Locals("role", user.Role)
		ctx.Locals("user_id", uid)
		log.Printf("AuthMiddleware - Usuario autenticado: %d, Path: %s", uid, ctx.Path())

		return ctx.Next()
	}
}
