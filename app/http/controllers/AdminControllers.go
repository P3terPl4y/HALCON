package controllers

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/csrf"
	"goravel/app/facades"
	"goravel/app/models"
	"goravel/app/requests"
	"goravel/app/services"
)

type AdminUserController struct {
	userService *services.UserService
}

func NewAdminUserController() *AdminUserController {
	return &AdminUserController{
		userService: services.NewUserService(),
	}
}

// Index - GET /admin/users
func (c *AdminUserController) Index(ctx fiber.Ctx) error {
	page := atoiDefault(ctx.Query("page"), 1)
	limit := atoiDefault(ctx.Query("limit"), 20)
	search := strings.TrimSpace(ctx.Query("search"))
	role := strings.TrimSpace(ctx.Query("role"))

	users, total, err := c.userService.AdminListUsers(page, limit, search, role)
	if err != nil {
		log.Printf("AdminUsers Index: %v", err)
		return ctx.Render("admin/users/index", fiber.Map{
			"title": "Usuarios",
			"error": "Error al cargar usuarios", "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	stats, _ := c.userService.AdminCountByRole()

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages < 1 {
		totalPages = 1
	}

	prevPage := page - 1
	if prevPage < 1 {
		prevPage = 1
	}
	nextPage := page + 1
	if nextPage > totalPages {
		nextPage = totalPages
	}

	return ctx.Render("admin/users/index", fiber.Map{
		"title":      "Usuarios",
		"users":      users,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
		"prevPage":   prevPage,
		"nextPage":   nextPage,
		"search":     search,
		"roleFilter": role,
		"stats":      stats, "csrfToken": csrf.TokenFromContext(ctx),
	}, "layouts/base")
}

// Create - GET /admin/users/create
func (c *AdminUserController) Create(ctx fiber.Ctx) error {
	return ctx.Render("admin/users/form", fiber.Map{
		"title": "Crear usuario",
		"mode":  "create", "csrfToken": csrf.TokenFromContext(ctx),
	}, "layouts/base")
}

// Store - POST /admin/users
func (c *AdminUserController) Store(ctx fiber.Ctx) error {
	var req requests.CreateUserRequest
	if err := ctx.Bind().Body(&req); err != nil {
		return ctx.Render("admin/users/form", fiber.Map{
			"title":       "Crear usuario",
			"mode":        "create",
			"flash_error": "Datos inválidos", "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	req.Role = strings.ToLower(strings.TrimSpace(req.Role))

	count, err := facades.Orm().Query().Model(&models.User{}).
		Where("email = ?", req.Email).Count()
	if err != nil {
		return ctx.Render("admin/users/form", fiber.Map{
			"title":       "Crear usuario",
			"mode":        "create",
			"flash_error": "Error al verificar el email",
			"old":         req, "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}
	if count > 0 {
		return ctx.Render("admin/users/form", fiber.Map{
			"title":       "Crear usuario",
			"mode":        "create",
			"flash_error": "El email ya está registrado",
			"old":         req, "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	user, err := c.userService.AdminCreateUser(&req)
	if err != nil {
		return ctx.Render("admin/users/form", fiber.Map{
			"title":       "Crear usuario",
			"mode":        "create",
			"flash_error": err.Error(),
			"old":         req, "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	facades.Log().Infof("Admin creó usuario ID=%d email=%s rol=%s", user.ID, user.Email, user.Role)
	return ctx.Redirect().To("/admin/users")
}

// Edit - GET /admin/users/:id/edit
func (c *AdminUserController) Edit(ctx fiber.Ctx) error {
	id := parseIDParam(ctx.Params("id"))
	if id == 0 {
		return ctx.Redirect().To("/admin/users")
	}

	user, err := c.userService.AdminGetUser(id)
	if err != nil {
		return ctx.Redirect().To("/admin/users")
	}

	return ctx.Render("admin/users/form", fiber.Map{
		"title": "Editar usuario",
		"mode":  "edit",
		"user":  user, "csrfToken": csrf.TokenFromContext(ctx),
	}, "layouts/base")
}

// Update - POST /admin/users/:id
func (c *AdminUserController) Update(ctx fiber.Ctx) error {
	id := parseIDParam(ctx.Params("id"))
	if id == 0 {
		return ctx.Redirect().To("/admin/users")
	}

	user, err := c.userService.AdminGetUser(id)
	if err != nil {
		return ctx.Redirect().To("/admin/users")
	}

	name := strings.TrimSpace(ctx.FormValue("name"))
	email := strings.ToLower(strings.TrimSpace(ctx.FormValue("email")))
	password := ctx.FormValue("password")
	role := strings.ToLower(strings.TrimSpace(ctx.FormValue("role")))

	updates, err := services.PrepareUserUpdates(requests.UserUpdateRequest{Name: name, Email: email, Password: password}, role)
	if err != nil {
		return ctx.Status(422).Render("admin/users/form", fiber.Map{"title": "Editar usuario", "mode": "edit", "user": user, "flash_error": err.Error(), "csrfToken": csrf.TokenFromContext(ctx)}, "layouts/base")
	}
	if normalized, ok := updates["email"].(string); ok && normalized != user.Email {
		count, err := facades.Orm().Query().Model(&models.User{}).
			Where("email = ?", normalized).
			Where("id <> ?", id).Count()
		if err != nil {
			return fiber.ErrInternalServerError
		}
		if count > 0 {
			return ctx.Render("admin/users/form", fiber.Map{
				"title":       "Editar usuario",
				"mode":        "edit",
				"user":        user,
				"flash_error": "El email ya está registrado", "csrfToken": csrf.TokenFromContext(ctx),
			}, "layouts/base")
		}
	}

	if err := c.userService.AdminUpdateUser(id, updates); err != nil {
		log.Printf("AdminUsers Update: %v", err)
		return ctx.Render("admin/users/form", fiber.Map{
			"title":       "Editar usuario",
			"mode":        "edit",
			"user":        user,
			"flash_error": "Error al actualizar el usuario", "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	return ctx.Redirect().To("/admin/users")
}

// Destroy - POST /admin/users/:id/delete
func (c *AdminUserController) Destroy(ctx fiber.Ctx) error {
	id := parseIDParam(ctx.Params("id"))
	if id == 0 {
		return ctx.Redirect().To("/admin/users")
	}

	currentID, _ := ctx.Locals("user_id").(uint)
	if currentID == id {
		return ctx.Redirect().To("/admin/users")
	}

	if err := c.userService.AdminDeleteUser(id); err != nil {
		log.Printf("AdminUsers Destroy: %v", err)
		return fiber.ErrInternalServerError
	}

	return ctx.Redirect().To("/admin/users")
}
