package controllers

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/csrf"
	"github.com/gofiber/fiber/v3/middleware/session"
	"golang.org/x/crypto/bcrypt"
	"goravel/app/facades"
	"goravel/app/http/middlewares"
	"goravel/app/models"
	"goravel/app/services"
)

type PublicUser struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

func publicUser(u *models.User) *PublicUser {
	if u == nil {
		return nil
	}
	return &PublicUser{u.ID, u.Name, u.Email, u.Role}
}

func SessionInfo(ctx fiber.Ctx) error {
	var user *models.User
	if sess := session.FromContext(ctx); sess != nil {
		if id, ok := middlewares.SessionUserID(sess.Get("user_id")); ok {
			u, err := services.NewUserService().GetByID(id)
			if err == nil && u.Status {
				user = u
			}
		}
	}
	ctx.Set("Cache-Control", "no-store")
	return ctx.JSON(fiber.Map{"user": publicUser(user), "csrf_token": csrf.TokenFromContext(ctx)})
}

func APILogin(ctx fiber.Ctx) error {
	var u models.User
	err := facades.Orm().Query().Where("email = ?", strings.ToLower(strings.TrimSpace(ctx.FormValue("email")))).FirstOrFail(&u)
	if err != nil || !u.Status || bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(ctx.FormValue("password"))) != nil {
		return ctx.Status(401).JSON(fiber.Map{"error": "Credenciales inválidas"})
	}
	sess := session.FromContext(ctx)
	if sess == nil {
		return fiber.ErrInternalServerError
	}
	if _, err := services.NewHalconService().EnsurePersonal(u.ID); err != nil {
		return fiber.ErrInternalServerError
	}
	if err := sess.Regenerate(); err != nil {
		return fiber.ErrInternalServerError
	}
	sess.Set("user_id", u.ID)
	sess.Set("role", u.Role)
	return SessionInfo(ctx)
}
func APILogout(ctx fiber.Ctx) error {
	if sess := session.FromContext(ctx); sess != nil {
		if err := sess.Destroy(); err != nil {
			return fiber.ErrInternalServerError
		}
	}
	return ctx.SendStatus(fiber.StatusNoContent)
}

func recipientFromRequest(ctx fiber.Ctx) (uint, error) {
	if raw := ctx.FormValue("recipient_id"); raw != "" {
		id, err := strconv.ParseUint(raw, 10, strconv.IntSize)
		return uint(id), err
	}
	var u models.User
	email := strings.ToLower(strings.TrimSpace(ctx.FormValue("recipient_email")))
	err := facades.Orm().Query().Where("LOWER(email) = ?", email).Where("status = ?", true).FirstOrFail(&u)
	return u.ID, err
}
func (c *UserController) Tracking(ctx fiber.Ctx) error {
	id := ctx.Locals("user_id").(uint)
	s := services.NewHalconService()
	personal, err := s.EnsurePersonal(id)
	if err != nil {
		return fiber.ErrInternalServerError
	}
	halcones, err := s.VisibleTo(id)
	if err != nil {
		return fiber.ErrInternalServerError
	}
	var recipient *models.User
	if personal.RecipientID != nil {
		recipient, _ = c.userService.GetByID(*personal.RecipientID)
	}
	return ctx.JSON(fiber.Map{"personal_id": personal.ID, "recipient": publicUser(recipient), "halcones": halcones, "csrf_token": csrf.TokenFromContext(ctx)})
}
func (c *UserController) ShareAPI(ctx fiber.Ctx) error {
	id, err := recipientFromRequest(ctx)
	if err != nil {
		return ctx.Status(422).JSON(fiber.Map{"error": "Indica el correo de una cuenta activa distinta a la tuya."})
	}
	if err := services.NewHalconService().SetRecipient(ctx.Locals("user_id").(uint), id); err != nil {
		return ctx.Status(422).JSON(fiber.Map{"error": "No se pudo compartir. Comprueba el destinatario."})
	}
	return c.Tracking(ctx)
}
