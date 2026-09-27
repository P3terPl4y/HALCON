package middlewares

import (
	"goravel/app/facades"
	"goravel/app/models"

	"github.com/gofiber/fiber/v3"
)

func ModeratorMiddleware() fiber.Handler {
	return func(ctx fiber.Ctx) error {
		userID, ok := ctx.Locals("user_id").(uint)
		if !ok {
			facades.Log().Warning("ModeratorMiddleware - user_id no encontrado en Locals")
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "No autenticado",
			})
		}

		var user models.User
		if err := facades.Orm().Query().Where("id = ?", userID).First(&user); err != nil {
			facades.Log().Errorf("ModeratorMiddleware - Usuario no encontrado: %d", userID)
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Usuario no encontrado",
			})
		}

		if user.Role != "moderator" && user.Role != "admin" {
			facades.Log().Warningf("ModeratorMiddleware - Acceso denegado para user %d con rol %s", userID, user.Role)
			return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Acceso denegado",
			})
		}

		facades.Log().Debugf("ModeratorMiddleware - Acceso permitido para user %d con rol %s", userID, user.Role)
		return ctx.Next()
	}
}
