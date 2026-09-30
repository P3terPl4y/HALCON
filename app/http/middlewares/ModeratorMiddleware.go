package middlewares

import (
	"github.com/gofiber/fiber/v3"
	"goravel/app/facades"
	"goravel/app/models"
	"log"
)

func ModeratorMiddleware() fiber.Handler {
	return func(ctx fiber.Ctx) error {
		userID, ok := ctx.Locals("user_id").(uint)
		if !ok {
			log.Printf("ModeratorMiddleware - user_id no encontrado en Locals")
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "No autenticado",
			})
		}

		var user models.User
		if err := facades.Orm().Query().Where("id = ?", userID).FirstOrFail(&user); err != nil || !user.Status {
			log.Printf("ModeratorMiddleware - Usuario no encontrado: %d", userID)
			return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Usuario no encontrado",
			})
		}

		if user.Role != "moderator" && user.Role != "admin" {
			log.Printf("ModeratorMiddleware - Acceso denegado para user %d con rol %s", userID, user.Role)
			return ctx.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Acceso denegado",
			})
		}

		log.Printf("ModeratorMiddleware - Acceso permitido para user %d con rol %s", userID, user.Role)
		return ctx.Next()
	}
}
