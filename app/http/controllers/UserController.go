package controllers

import (
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/csrf"
	"golang.org/x/crypto/bcrypt"
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

	users, total, err := c.userService.GetAll(page, limit)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error al obtener los usuarios",
		})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"users": users,
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

	updates := map[string]any{}

	if req.Name != "" {
		updates["name"] = req.Name
	}

	if req.Email != "" && req.Email != user.Email {
		count, _ := facades.Orm().Query().Model(&models.User{}).
			Where("email = ?", req.Email).
			Where("id <> ?", user.ID).
			Count()
		if count > 0 {
			return ctx.Render("profile/edit", fiber.Map{
				"title":       "Editar Perfil",
				"flash_error": "El email ya está registrado",
				"user":        user, "csrfToken": csrf.TokenFromContext(ctx),
			}, "layouts/base")
		}
		updates["email"] = req.Email
	}

	if req.Password != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return ctx.Render("profile/edit", fiber.Map{
				"title":       "Editar Perfil",
				"flash_error": "Error al procesar la contraseña",
				"user":        user, "csrfToken": csrf.TokenFromContext(ctx),
			}, "layouts/base")
		}
		updates["password"] = string(hashed)
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
