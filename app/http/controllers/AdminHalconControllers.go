package controllers

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/csrf"
	"goravel/app/facades"
	"goravel/app/requests"
	"goravel/app/services"
	"strconv"
)

type AdminHalconController struct {
	halconService *services.HalconService
	userService   *services.UserService
}

func NewAdminHalconController() *AdminHalconController {
	return &AdminHalconController{
		halconService: services.NewHalconService(),
		userService:   services.NewUserService(),
	}
}

// Index - GET /admin/halcones
func (c *AdminHalconController) Index(ctx fiber.Ctx) error {
	page := atoiDefault(ctx.Query("page"), 1)
	limit := atoiDefault(ctx.Query("limit"), 20)
	search := strings.TrimSpace(ctx.Query("search"))
	active := ctx.Query("active")

	halcones, total, err := c.halconService.AdminListHalcones(page, limit, search, active)
	if err != nil {
		log.Printf("AdminHalcones Index: %v", err)
		return ctx.Render("admin/halcones/index", fiber.Map{
			"title": "Halcones",
			"error": "Error al cargar halcones", "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	stats, _ := c.halconService.AdminCountStats()

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages < 1 {
		totalPages = 1
	}

	return ctx.Render("admin/halcones/index", fiber.Map{
		"title":      "Halcones",
		"halcones":   halcones,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
		"search":     search,
		"active":     active,
		"stats":      stats,
		"csrfToken":  csrf.TokenFromContext(ctx),
	}, "layouts/base")
}

// Create - GET /admin/halcones/create
func (c *AdminHalconController) Create(ctx fiber.Ctx) error {
	return ctx.Render("admin/halcones/form", fiber.Map{
		"title": "Crear halcón",
		"mode":  "create", "csrfToken": csrf.TokenFromContext(ctx),
	}, "layouts/base")
}

// Store - POST /admin/halcones
func (c *AdminHalconController) Store(ctx fiber.Ctx) error {
	name := strings.TrimSpace(ctx.FormValue("name"))
	if name == "" {
		return ctx.Render("admin/halcones/form", fiber.Map{
			"title":       "Crear halcón",
			"mode":        "create",
			"flash_error": "El nombre es obligatorio", "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	req := requests.CreateHalconRequest{Name: name, ModeratorID: ctx.Locals("user_id").(uint)}
	halcon, err := c.halconService.Create(&req)
	if err != nil {
		return ctx.Render("admin/halcones/form", fiber.Map{
			"title":       "Crear halcón",
			"mode":        "create",
			"flash_error": err.Error(),
			"old":         req, "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	facades.Log().Infof("Admin creó halcón ID=%d name=%s", halcon.ID, halcon.Name)
	// Pasar el token en la URL para mostrarlo una sola vez
	return ctx.Redirect().To("/admin/halcones/" + strconv.FormatUint(uint64(halcon.ID), 10) + "?created=1")
}

// Show - GET /admin/halcones/:id
func (c *AdminHalconController) Show(ctx fiber.Ctx) error {
	id := parseIDParam(ctx.Params("id"))
	if id == 0 {
		return ctx.Redirect().To("/admin/halcones")
	}

	halcon, err := c.halconService.AdminGetHalcon(id)
	if err != nil {
		return ctx.Redirect().To("/admin/halcones")
	}

	assignments, _ := c.halconService.AdminGetAssignmentsHistory(id)

	// Si acaba de crearse, pasar el token para mostrarlo una vez
	var createdToken string
	if ctx.Query("created") == "1" {
		createdToken = halcon.Token
	}

	return ctx.Render("admin/halcones/show", fiber.Map{
		"title":        "Detalle del halcón",
		"halcon":       halcon,
		"assignments":  assignments,
		"createdToken": createdToken, "csrfToken": csrf.TokenFromContext(ctx),
	}, "layouts/base")
}

// Edit - GET /admin/halcones/:id/edit
func (c *AdminHalconController) Edit(ctx fiber.Ctx) error {
	id := parseIDParam(ctx.Params("id"))
	if id == 0 {
		return ctx.Redirect().To("/admin/halcones")
	}

	halcon, err := c.halconService.GetByID(id)
	if err != nil {
		return ctx.Redirect().To("/admin/halcones")
	}

	return ctx.Render("admin/halcones/form", fiber.Map{
		"title":  "Editar halcón",
		"mode":   "edit",
		"halcon": halcon, "csrfToken": csrf.TokenFromContext(ctx),
	}, "layouts/base")
}

// Update - POST /admin/halcones/:id
func (c *AdminHalconController) Update(ctx fiber.Ctx) error {
	id := parseIDParam(ctx.Params("id"))
	if id == 0 {
		return ctx.Redirect().To("/admin/halcones")
	}

	name := strings.TrimSpace(ctx.FormValue("name"))
	isActive := ctx.FormValue("is_active") == "on" || ctx.FormValue("is_active") == "1"

	_, err := c.halconService.AdminUpdateHalcon(id, name, isActive)
	if err != nil {
		halcon, _ := c.halconService.GetByID(id)
		return ctx.Render("admin/halcones/form", fiber.Map{
			"title":       "Editar halcón",
			"mode":        "edit",
			"halcon":      halcon,
			"flash_error": "Error al actualizar", "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	return ctx.Redirect().To("/admin/halcones")
}

// Destroy - POST /admin/halcones/:id/delete
func (c *AdminHalconController) Destroy(ctx fiber.Ctx) error {
	id := parseIDParam(ctx.Params("id"))
	if id == 0 {
		return ctx.Redirect().To("/admin/halcones")
	}

	if err := c.halconService.AdminDeleteHalcon(id); err != nil {
		log.Printf("AdminHalcones Destroy: %v", err)
	}

	return ctx.Redirect().To("/admin/halcones")
}

// AssignForm - GET /admin/halcones/:id/assign
func (c *AdminHalconController) AssignForm(ctx fiber.Ctx) error {
	id := parseIDParam(ctx.Params("id"))
	if id == 0 {
		return ctx.Redirect().To("/admin/halcones")
	}

	halcon, err := c.halconService.GetByID(id)
	if err != nil {
		return ctx.Redirect().To("/admin/halcones")
	}

	// Cargar usuarios (solo rol user) para el selector
	users, _, _ := c.userService.SearchAssignable(1, 100, "")

	return ctx.Render("admin/halcones/assign", fiber.Map{
		"title":  "Asignar halcón",
		"halcon": halcon,
		"users":  users, "csrfToken": csrf.TokenFromContext(ctx),
	}, "layouts/base")
}

// AssignStore - POST /admin/halcones/:id/assign
func (c *AdminHalconController) AssignStore(ctx fiber.Ctx) error {
	id := parseIDParam(ctx.Params("id"))
	if id == 0 {
		return ctx.Redirect().To("/admin/halcones")
	}

	userID := parseIDParam(ctx.FormValue("user_id"))
	packageID := strings.TrimSpace(ctx.FormValue("package_id"))

	if userID == 0 {
		halcon, _ := c.halconService.GetByID(id)
		users, _, _ := c.userService.SearchAssignable(1, 100, "")
		return ctx.Render("admin/halcones/assign", fiber.Map{
			"title":       "Asignar halcón",
			"halcon":      halcon,
			"users":       users,
			"flash_error": "Selecciona un usuario válido", "csrfToken": csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	moderatorID, _ := ctx.Locals("user_id").(uint)

	_, err := c.halconService.Assign(id, userID, moderatorID, packageID)
	if err != nil {
		log.Printf("AdminHalcones Assign: %v", err)
		return ctx.Redirect().To("/admin/halcones")
	}

	return ctx.Redirect().To("/admin/halcones/" + ctx.Params("id"))
}
