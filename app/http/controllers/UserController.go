package controllers

import (
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/csrf"
	"goravel/app/facades"
	"goravel/app/models"
	"goravel/app/requests"
	"goravel/app/services"
)

type UserController struct {
	userService *services.UserService
}

func NewUserController() *UserController {
	return &UserController{
		userService: services.NewUserService(),
	}
}

// getCurrentUserID extrae el user_id de Locals.
func (c *UserController) getCurrentUserID(ctx fiber.Ctx) (uint, bool) {
	id, ok := ctx.Locals("user_id").(uint)
	return id, ok
}

// Index - GET /users
func (c *UserController) Index(ctx fiber.Ctx) error {
	page := atoiDefault(ctx.Query("page"), 1)
	limit := atoiDefault(ctx.Query("limit"), 20)

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	users, total, err := c.userService.SearchAssignable(page, limit, ctx.Query("search"))
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error al obtener los usuarios",
		})
	}
	public := make([]*PublicUser, 0, len(users))
	for i := range users {
		public = append(public, publicUser(&users[i]))
	}
	ctx.Set("Cache-Control", "no-store")
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"users": public,
		"total": total,
		"page":  page,
		"limit": limit, "csrfToken": csrf.TokenFromContext(ctx),
	})
}

// Show - GET /profile
func (c *UserController) Show(ctx fiber.Ctx) error {
	userID, ok := c.getCurrentUserID(ctx)
	if !ok {
		return ctx.Redirect().To("/login")
	}

	user, err := c.userService.GetByID(userID)
	if err != nil {
		log.Printf("Error al obtener perfil: %v", err)
		return ctx.Render("auth/login", fiber.Map{
			"title":       "Iniciar sesión",
			"flash_error": "Usuario no encontrado", "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	// Obtener halcones asignados al usuario
	halconService := services.NewHalconService()
	halcones, err := halconService.VisibleTo(userID)
	if err != nil {
		log.Printf("Error al obtener halcones: %v", err)
		return fiber.ErrInternalServerError
	}

	personal, err := halconService.EnsurePersonal(userID)
	if err != nil {
		return fiber.ErrInternalServerError
	}
	var recipient *models.User
	if personal.RecipientID != nil {
		recipient, _ = c.userService.GetByID(*personal.RecipientID)
	}
	return ctx.Render("profile/show", fiber.Map{
		"recipient": recipient, "shareSuccess": ctx.Query("ok") == "destinatario",
		"personal": personal, "shareError": ctx.Query("error"),
		"title":    "Mi Perfil",
		"user":     user,
		"halcones": halcones, "csrfToken": csrf.TokenFromContext(ctx),
	}, "layouts/base")
}

// Edit - GET /profile/edit
func (c *UserController) Edit(ctx fiber.Ctx) error {
	userID, ok := c.getCurrentUserID(ctx)
	if !ok {
		return ctx.Redirect().To("/login")
	}

	user, err := c.userService.GetByID(userID)
	if err != nil {
		log.Printf("Error al obtener perfil: %v", err)
		return ctx.Render("profile/edit", fiber.Map{
			"title":       "Editar Perfil",
			"flash_error": "Usuario no encontrado", "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	return ctx.Render("profile/edit", fiber.Map{
		"title": "Editar Perfil",
		"user":  user, "csrfToken": csrf.TokenFromContext(ctx),
	}, "layouts/base")
}

// Update - POST /profile/edit
func (c *UserController) Update(ctx fiber.Ctx) error {
	userID, ok := c.getCurrentUserID(ctx)
	if !ok {
		return ctx.Redirect().To("/login")
	}

	user, err := c.userService.GetByID(userID)
	if err != nil {
		return ctx.Redirect().To("/login")
	}

	var req requests.UserUpdateRequest
	if err := ctx.Bind().Body(&req); err != nil {
		return ctx.Render("profile/edit", fiber.Map{
			"title":       "Editar Perfil",
			"flash_error": "Datos inválidos",
			"user":        user, "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	updates, err := services.PrepareUserUpdates(req, "")
	if err != nil {
		return ctx.Status(422).Render("profile/edit", fiber.Map{"title": "Editar Perfil", "flash_error": err.Error(), "user": user, "csrfToken": csrf.TokenFromContext(ctx)}, "layouts/base")
	}
	if email, ok := updates["email"].(string); ok && email != user.Email {
		count, err := facades.Orm().Query().Model(&models.User{}).
			Where("email = ?", email).
			Where("id <> ?", user.ID).
			Count()
		if err != nil {
			return fiber.ErrInternalServerError
		}
		if count > 0 {
			return ctx.Render("profile/edit", fiber.Map{
				"title":       "Editar Perfil",
				"flash_error": "El email ya está registrado",
				"user":        user, "csrfToken": csrf.TokenFromContext(ctx),
			}, "layouts/base")
		}
	}

	if len(updates) > 0 {
		if err := c.userService.Update(userID, updates); err != nil {
			log.Printf("Error al actualizar perfil: %v", err)
			return ctx.Render("profile/edit", fiber.Map{
				"title":       "Editar Perfil",
				"flash_error": "Error al actualizar el perfil",
				"user":        user, "csrfToken": csrf.TokenFromContext(ctx),
			}, "layouts/base")
		}
	}

	return ctx.Redirect().To("/profile")
}
