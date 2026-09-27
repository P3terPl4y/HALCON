package middlewares

import (
	"goravel/app/facades"
	"goravel/app/models"

	"github.com/gofiber/fiber/v3"
)

// AdminMiddleware verifica que el usuario autenticado tenga rol "admin".
// Debe ejecutarse después de AuthMiddleware (que puebla Locals["user_id"]).
func AdminMiddleware() fiber.Handler {
	return func(ctx fiber.Ctx) error {
		userID, ok := ctx.Locals("user_id").(uint)
		if !ok {
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "No autenticado",
			})
		}

		var user models.User
		if err := facades.Orm().Query().Where("id = ?", userID).First(&user); err != nil {
			facades.Log().Errorf("AdminMiddleware: usuario %d no encontrado", userID)
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Usuario no encontrado",
			})
		}

		if user.Role != "admin" {
			facades.Log().Warningf("AdminMiddleware: acceso denegado para user %d (rol=%s)", userID, user.Role)
			return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Acceso denegado. Se requiere rol administrador.",
			})
		}

		ctx.Locals("role", user.Role)
		return ctx.Next()
	}
}
