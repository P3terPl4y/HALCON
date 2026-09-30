package controllers

import (
	"github.com/gofiber/fiber/v3"
	"goravel/app/services"
)

func (c *UserController) Share(ctx fiber.Ctx) error {
	id, err := recipientFromRequest(ctx)
	if err != nil {
		return ctx.Redirect().To("/profile?error=destinatario_invalido")
	}
	if err := services.NewHalconService().SetRecipient(ctx.Locals("user_id").(uint), id); err != nil {
		return ctx.Redirect().To("/profile?error=destinatario_invalido")
	}
	return ctx.Redirect().To("/profile?ok=destinatario")
}
