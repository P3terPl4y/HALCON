package controllers

import (
	"strconv"
	"strings"
"log"
	"goravel/app/requests"
	"goravel/app/services"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/csrf"
)

type HalconController struct {
	halconService *services.HalconService
	userService   *services.UserService
}

func NewHalconController() *HalconController {
	return &HalconController{
		halconService: services.NewHalconService(),
		userService:   services.NewUserService(),
	}
}

// Index - GET /moderator/halcones
func (c *HalconController) Index(ctx fiber.Ctx) error {
	moderatorID, _ := ctx.Locals("user_id").(uint)
	role, _ := ctx.Locals("role").(string)
	isAdmin := role == "admin"

	page := atoiDefault(ctx.Query("page"), 1)
	limit := atoiDefault(ctx.Query("limit"), 20)
	search := strings.TrimSpace(ctx.Query("search"))
	active := ctx.Query("active")

	halcones, total, err := c.halconService.ListByModerator(
		moderatorID, page, limit, search, active, isAdmin,
	)
	if err != nil {
		return ctx.Render("moderator/halcones/index", fiber.Map{
			"title":         "Mis halcones",
			"flash_error":   "Error al cargar los halcones",
			"csrfToken":     csrf.TokenFromContext(ctx),
		}, "layouts/base")
	}

	users, _, _ := c.userService.AdminListUsers(1, 200, "", "user")
	stats, _ := c.halconService.StatsByModerator(moderatorID, isAdmin)

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

	// Mensajes flash vía query params
	flashError := ""
	flashSuccess := ""
	switch ctx.Query("error") {
	case "nombre_vacio":
		flashError = "El nombre del halcón es obligatorio."
	case "no_se_pudo_crear":
		flashError = "No se pudo crear el halcón. Intenta de nuevo."
	case "no_se_pudo_asignar":
		flashError = "No se pudo asignar el halcón."
	case "no_se_pudo_eliminar":
		flashError = "No se pudo eliminar el halcón."
	case "datos_invalidos":
		flashError = "Datos inválidos."
	}
	switch ctx.Query("ok") {
	case "creado":
		flashSuccess = "Halcón creado correctamente."
	case "asignado":
		flashSuccess = "Halcón asignado al usuario."
	case "eliminado":
		flashSuccess = "Halcón eliminado."
	}

	return ctx.Render("moderator/halcones/index", fiber.Map{
		"title":         "Mis halcones",
		"csrfToken":     csrf.TokenFromContext(ctx),
		"halcones":      halcones,
		"users":         users,
		"stats":         stats,
		"total":         total,
		"page":          page,
		"limit":         limit,
		"totalPages":    totalPages,
		"prevPage":      prevPage,
		"nextPage":      nextPage,
		"search":        search,
		"active":        active,
		"flash_error":   flashError,
		"flash_success": flashSuccess,
	}, "layouts/base")
}

// Store - POST /moderator/halcones
func (c *HalconController) Store(ctx fiber.Ctx) error {
	name := strings.TrimSpace(ctx.FormValue("name"))
	if name == "" {
		return ctx.Redirect().To("/moderator/halcones?error=nombre_vacio")
	}

	moderatorID, _ := ctx.Locals("user_id").(uint)

	req := requests.CreateHalconRequest{
		Name:        name,
		ModeratorID: moderatorID,
	}
	if _, err := c.halconService.Create(&req); err != nil {
		return ctx.Redirect().To("/moderator/halcones?error=no_se_pudo_crear")
	}

	return ctx.Redirect().To("/moderator/halcones?ok=creado")
}

// Assign - POST /moderator/halcones/assign
func (c *HalconController) Assign(ctx fiber.Ctx) error {
	halconID := parseIDParam(ctx.FormValue("halcon_id"))
	userID := parseIDParam(ctx.FormValue("user_id"))
	packageID := strings.TrimSpace(ctx.FormValue("package_id"))

	if halconID == 0 || userID == 0 {
		return ctx.Redirect().To("/moderator/halcones?error=datos_invalidos")
	}

	moderatorID, _ := ctx.Locals("user_id").(uint)

	if _, err := c.halconService.Assign(halconID, userID, moderatorID, packageID); err != nil {
		return ctx.Redirect().To("/moderator/halcones?error=no_se_pudo_asignar")
	}

	return ctx.Redirect().To("/moderator/halcones?ok=asignado")
}

// Destroy - DELETE /moderator/halcones/:id
func (c *HalconController) Destroy(ctx fiber.Ctx) error {
	id := parseIDParam(ctx.Params("id"))
	if id == 0 {
		return ctx.Redirect().To("/moderator/halcones?error=datos_invalidos")
	}

	if err := c.halconService.Delete(id); err != nil {
		return ctx.Redirect().To("/moderator/halcones?error=no_se_pudo_eliminar")
	}

	return ctx.Redirect().To("/moderator/halcones?ok=eliminado")
}

// Evita el import no usado cuando Go no detecta el uso
var _ = strconv.Itoa
// Editor - GET /moderator/halcones/:id/editor
// Muestra la vista de edición/simulación de un halcón propio del moderador.
func (c *HalconController) Editor(ctx fiber.Ctx) error {
	id := parseIDParam(ctx.Params("id"))
	if id == 0 {
		return ctx.Redirect().To("/moderator/halcones?error=datos_invalidos")
	}

	moderatorID, _ := ctx.Locals("user_id").(uint)
	role, _ := ctx.Locals("role").(string)
	isAdmin := role == "admin"

	halcon, err := c.halconService.GetByID(id)
	if err != nil {
		return ctx.Redirect().To("/moderator/halcones?error=no_encontrado")
	}

	// Verificación de propiedad: el moderador solo puede ver sus propios halcones
	if !isAdmin && halcon.ModeratorID != moderatorID {
		log.Printf("moderator %d intentó acceder al halcón %d (dueño: %d)",
			moderatorID, halcon.ID, halcon.ModeratorID)
		return ctx.Redirect().To("/moderator/halcones?error=sin_permiso")
	}

	// Asignación activa (puede no existir)
	var assignedUserID uint
	var assignedUserName string
	if a, err := c.halconService.GetActiveAssignment(id); err == nil {
		assignedUserID = a.UserID
		if u, err := c.userService.AdminGetUser(a.UserID); err == nil {
			assignedUserName = u.Name
		}
	}

	return ctx.Render("moderator/halcones/editor", fiber.Map{
		"title":            "Editar halcón · " + halcon.Name,
		"csrfToken":        csrf.TokenFromContext(ctx),
		"halcon":           halcon,
		"assignedUserID":   assignedUserID,
		"assignedUserName": assignedUserName,
		"hasAssignment":    assignedUserID > 0,
	}, "layouts/base")
}
