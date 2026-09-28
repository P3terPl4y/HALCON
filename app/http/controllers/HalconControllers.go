package controllers

import (
	"strings"

	

	"goravel/app/requests"
	"goravel/app/services"
"github.com/gofiber/fiber/v3/middleware/csrf"
	"github.com/gofiber/fiber/v3"
)

type HalconController struct {
	halconService *services.HalconService
	userService *services.UserService
}

func NewHalconController() *HalconController {
	return &HalconController{
		halconService: services.NewHalconService(),
		userService:services.NewUserService(),
	}
}

func (c *HalconController) Index(ctx fiber.Ctx) error {
    moderatorID, _ := ctx.Locals("user_id").(uint)
    role, _ := ctx.Locals("role").(string)

    page := atoiDefault(ctx.Query("page"), 1)
    limit := atoiDefault(ctx.Query("limit"), 20)
    search := strings.TrimSpace(ctx.Query("search"))
    active := ctx.Query("active")

    // Lista de halcones del moderador (con la asignación activa cargada)
    halcones, total, err := c.halconService.ListByModerator(moderatorID, page, limit, search, active, role == "admin")
    if err != nil {
        return ctx.Render("moderator/halcones/index", fiber.Map{
            "title": "Mis halcones",
            "flash_error": "Error al cargar los halcones",
        }, "layouts/base")
    }

    // Usuarios disponibles para asignar (solo rol "user")
    users, _, _ := c.userService.AdminListUsers(1, 200, "", "user")
    
    isAdmin:=false
	if role=="admin"{
		isAdmin=true
	}
    stats, _ := c.halconService.StatsByModerator(moderatorID,isAdmin)

    totalPages := int((total + int64(limit) - 1) / int64(limit))
    if totalPages < 1 { totalPages = 1 }

    prevPage := page - 1
    if prevPage < 1 { prevPage = 1 }
    nextPage := page + 1
    if nextPage > totalPages { nextPage = totalPages }

    return ctx.Render("moderator/halcones/index", fiber.Map{
        "title":      "Mis halcones",
        "moderator_id":moderatorID,
        "halcones":   halcones,   // []HalconWithAssignment
        "users":      users,      // []models.User
        "stats":      stats,      // map[string]int64{"total", "active", "assigned"}
        "total":      total,
        "page":       page,
        "limit":      limit,
        "totalPages": totalPages,
        "prevPage":   prevPage,
        "nextPage":   nextPage,
        "search":     search,
        "active":     active,
        "flash_error":   ctx.Query("error"),
        "flash_success": ctx.Query("ok"),
        "csrfToken":    csrf.TokenFromContext(ctx),
    }, "layouts/base")
}

// Store - POST /moderator/halcones
func (c *HalconController) Store(ctx fiber.Ctx) error {
	var req requests.CreateHalconRequest
	if err := ctx.Bind().Body(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Datos inválidos"})
	}

	halcon, err := c.halconService.Create(&req)
	if err != nil {
		return ctx.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{"halcon": halcon,"csrfToken":    csrf.TokenFromContext(ctx)})
}

// Assign - POST /moderator/halcones/assign
func (c *HalconController) Assign(ctx fiber.Ctx) error {
	var req requests.AssignHalconRequest
	if err := ctx.Bind().Body(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Datos inválidos","csrfToken":    csrf.TokenFromContext(ctx)})
	}

	moderatorID, _ := ctx.Locals("user_id").(uint)

	assignment, err := c.halconService.Assign(req.HalconID, req.UserID, moderatorID, req.PackageID)
	if err != nil {
		return ctx.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"assignment": assignment})
}

// Destroy - DELETE /moderator/halcones/:id
func (c *HalconController) Destroy(ctx fiber.Ctx) error {
	id := parseIDParam(ctx.Params("id"))
	if id == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "ID inválido"})
	}

	if err := c.halconService.Delete(id); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error(),"csrfToken":    csrf.TokenFromContext(ctx)})
	}
	return ctx.Status(fiber.StatusNoContent).Send(nil)
}
