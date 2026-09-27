package controllers

import (
	"log"

	"goravel/app/facades"
	"goravel/app/models"
	"goravel/app/requests"
	"goravel/app/services"

	"github.com/gofiber/fiber/v3"
)

type HalconController struct {
	halconService *services.HalconService
}

func NewHalconController() *HalconController {
	return &HalconController{
		halconService: services.NewHalconService(),
	}
}

// Index - GET /moderator/halcones
func (c *HalconController) Index(ctx fiber.Ctx) error {
	moderatorID, _ := ctx.Locals("user_id").(uint)
	role, _ := ctx.Locals("role").(string)

	var halcones []models.Halcon
	var err error

	if role == "admin" {
		err = facades.Orm().Query().Model(&models.Halcon{}).Find(&halcones)
	} else {
		halcones, err = c.halconService.GetByModeratorID(moderatorID)
	}

	if err != nil {
		log.Printf("Error al listar halcones: %v", err)
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Error al obtener halcones",
		})
	}
	return ctx.JSON(fiber.Map{"halcones": halcones})
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
	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{"halcon": halcon})
}

// Assign - POST /moderator/halcones/assign
func (c *HalconController) Assign(ctx fiber.Ctx) error {
	var req requests.AssignHalconRequest
	if err := ctx.Bind().Body(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Datos inválidos"})
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
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusNoContent).Send(nil)
}
