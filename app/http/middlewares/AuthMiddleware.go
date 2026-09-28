package middlewares

import (
	"goravel/app/facades"
"log"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
)

func AuthMiddleware() fiber.Handler {
	return func(ctx fiber.Ctx) error {
		sess := session.FromContext(ctx)
		if sess == nil {
			log.Printf("AuthMiddleware - Sesión nil en %s", ctx.Path())
			return ctx.Redirect().To("/login")
		}

		userID := sess.Get("user_id")
		if userID == nil {
			log.Printf("AuthMiddleware - user_id no encontrado en sesión para %s", ctx.Path())
			return ctx.Redirect().To("/login")
		}

		// Convertir a uint (Fiber serializa como int64/float64 según el store).
		var uid uint
		switch v := userID.(type) {
		case uint:
			uid = v
		case int:
			uid = uint(v)
		case int64:
			uid = uint(v)
		case float64:
			uid = uint(v)
		default:
			facades.Log().Errorf("AuthMiddleware - Tipo de user_id no soportado: %T", userID)
			return ctx.Redirect().To("/login")
		}

		ctx.Locals("user_id", uid)
		log.Printf("AuthMiddleware - Usuario autenticado: %d, Path: %s", uid, ctx.Path())

		if role, ok := sess.Get("role").(string); ok {
			ctx.Locals("role", role)
		}

		return ctx.Next()
	}
}
