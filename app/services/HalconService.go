package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"goravel/app/facades"
	"goravel/app/models"
	"goravel/app/requests"

	"github.com/goravel/framework/contracts/database/orm"
)

type HalconService struct{}
// app/services/halcon_service.go (o donde tengas el DTO)


func NewHalconService() *HalconService { return &HalconService{} }

// generateToken crea un token aleatorio de 32 bytes (64 hex chars).
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Create crea un halcón SIN asignar. La asignación es un paso separado.
func (s *HalconService) Create(req *requests.CreateHalconRequest) (*models.Halcon, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generar token: %w", err)
	}

	halcon := models.Halcon{
		Name:     req.Name,
		ModeratorID: req.ModeratorID,
		Token:    token,
		IsActive: false,
	}
	if err := facades.Orm().Query().Create(&halcon); err != nil {
		return nil, fmt.Errorf("crear halcón: %w", err)
	}
	return &halcon, nil
}

// GetByID obtiene un halcón por ID.
func (s *HalconService) GetByID(id uint) (*models.Halcon, error) {
	var h models.Halcon
	if err := facades.Orm().Query().Where("id = ?", id).First(&h); err != nil {
		return nil, errors.New("halcón no encontrado")
	}
	return &h, nil
}

// GetByModeratorID devuelve los halcones asignados por un moderador.
func (s *HalconService) GetByModeratorID(moderatorID uint) ([]models.Halcon, error) {
	var halcones []models.Halcon
	err := facades.Orm().Query().
		Model(&models.Halcon{}).
		Join("JOIN halcon_assignments ON halcon_assignments.halcon_id = halcones.id").
		Where("halcon_assignments.moderator_id = ?", moderatorID).
		Where("halcon_assignments.ended_at IS NULL").
		Find(&halcones)
	return halcones, err
}

// GetByAssignedUserID devuelve los halcones actualmente asignados a un usuario.
func (s *HalconService) GetByAssignedUserID(userID uint) ([]models.Halcon, error) {
	var halcones []models.Halcon
	err := facades.Orm().Query().
		Model(&models.Halcon{}).
		Join("JOIN halcon_assignments ON halcon_assignments.halcon_id = halcones.id").
		Where("halcon_assignments.user_id = ?", userID).
		Where("halcon_assignments.ended_at IS NULL").
		Find(&halcones)
	return halcones, err
}

// ValidateToken valida el token de un halcón.
func (s *HalconService) ValidateToken(token string) (*models.Halcon, error) {
	var h models.Halcon
	if err := facades.Orm().Query().Where("token = ?", token).First(&h); err != nil {
		return nil, errors.New("credenciales de halcón inválidas")
	}
	return &h, nil
}

// GetActiveAssignment obtiene la asignación activa de un halcón.
func (s *HalconService) GetActiveAssignment(halconID uint) (*models.HalconAssignment, error) {
	var a models.HalconAssignment
	if err := facades.Orm().Query().
		Where("halcon_id = ?", halconID).
		Where("ended_at IS NULL").
		Order("assigned_at DESC").
		First(&a); err != nil {
		return nil, errors.New("halcón no asignado a ningún usuario")
	}
	return &a, nil
}

// Assign asigna un halcón a un usuario. Cierra la asignación anterior si existe.
func (s *HalconService) Assign(halconID, userID, moderatorID uint, packageID string) (*models.HalconAssignment, error) {
	var assignment *models.HalconAssignment

	err := facades.Orm().Transaction(func(tx orm.Query) error {
		now := time.Now()

		// Cerrar asignación anterior si existe
		if _, err := tx.Model(&models.HalconAssignment{}).
			Where("halcon_id = ?", halconID).
			Where("ended_at IS NULL").
			Update(map[string]any{"ended_at": now}); err != nil {
			return fmt.Errorf("cerrar asignación anterior: %w", err)
		}

		// Crear nueva asignación
		a := models.HalconAssignment{
			HalconID:    halconID,
			UserID:      userID,
			ModeratorID: moderatorID,
			PackageID:   packageID,
			AssignedAt:  now,
		}
		if err := tx.Create(&a); err != nil {
			return fmt.Errorf("crear asignación: %w", err)
		}

		assignment = &a
		return nil
	})

	if err != nil {
		return nil, err
	}
	return assignment, nil
}

// Activate marca el halcón como activo.
func (s *HalconService) Activate(id uint) error {
	now := time.Now()
	_, err := facades.Orm().Query().Model(&models.Halcon{}).
		Where("id = ?", id).
		Update(map[string]any{
			"is_active": true,
			"last_seen": now,
		})
	return err
}

// Deactivate marca el halcón como inactivo.
func (s *HalconService) Deactivate(id uint) error {
	_, err := facades.Orm().Query().Model(&models.Halcon{}).
		Where("id = ?", id).
		Update("is_active", false)
	return err
}

// UpdateLastLocation actualiza la última posición conocida.
func (s *HalconService) UpdateLastLocation(id uint, lat, lng float64) error {
	now := time.Now()
	_, err := facades.Orm().Query().Model(&models.Halcon{}).
		Where("id = ?", id).
		Update(map[string]any{
			"last_lat":  lat,
			"last_lng":  lng,
			"last_seen": now,
		})
	return err
}

// Update actualiza los campos editables del halcón.
func (s *HalconService) Update(id uint, req *requests.UpdateHalconRequest) (*models.Halcon, error) {
	if _, err := facades.Orm().Query().Model(&models.Halcon{}).
		Where("id = ?", id).Update("name", req.Name); err != nil {
		return nil, err
	}
	return s.GetByID(id)
}

// Delete elimina un halcón y sus asignaciones.
func (s *HalconService) Delete(id uint) error {
	return facades.Orm().Transaction(func(tx orm.Query) error {
		if _, err := tx.Where("halcon_id = ?", id).Delete(&models.HalconAssignment{}); err != nil {
			return err
		}
		if _, err := tx.Where("id = ?", id).Delete(&models.Halcon{}); err != nil {
			return err
		}
		return nil
	})
}

// Count devuelve el total de halcones.
func (s *HalconService) Count() (int64, error) {
	return facades.Orm().Query().Model(&models.Halcon{}).Count()
}
// ============================================================
// ADMIN — CRUD de halcones
// ============================================================

// AdminListHalcones devuelve halcones paginados con filtros.
func (s *HalconService) AdminListHalcones(page, limit int, search, active string) ([]models.Halcon, int64, error) {
	if page < 1 { page = 1 }
	if limit < 1 || limit > 100 { limit = 20 }

	query := facades.Orm().Query().Model(&models.Halcon{})

	if search != "" {
		like := "%" + search + "%"
		query = query.Where("name LIKE ?", like)
	}
	if active == "1" {
		query = query.Where("is_active = ?", true)
	} else if active == "0" {
		query = query.Where("is_active = ?", false)
	}

	var halcones []models.Halcon
	var total int64
	if err := query.Paginate(page, limit, &halcones, &total); err != nil {
		return nil, 0, err
	}
	return halcones, total, nil
}

// AdminGetHalcon obtiene un halcón por ID con sus asignaciones.
func (s *HalconService) AdminGetHalcon(id uint) (*models.Halcon, error) {
	var h models.Halcon
	if err := facades.Orm().Query().With("Halcon").Where("id = ?", id).FirstOrFail(&h); err != nil {
		// Nota: el With no aplica porque no hay relación directa; se mantiene por consistencia
		if err := facades.Orm().Query().Where("id = ?", id).FirstOrFail(&h); err != nil {
			return nil, errors.New("halcón no encontrado")
		}
	}
	return &h, nil
}

// AdminUpdateHalcon actualiza nombre y estado activo.
func (s *HalconService) AdminUpdateHalcon(id uint, name string, isActive bool) (*models.Halcon, error) {
	_, err := facades.Orm().Query().Model(&models.Halcon{}).
		Where("id = ?", id).
		Update(map[string]any{
			"name":      name,
			"is_active": isActive,
		})
	if err != nil { return nil, err }
	return s.GetByID(id)
}

// AdminDeleteHalcon elimina un halcón y sus asignaciones.
func (s *HalconService) AdminDeleteHalcon(id uint) error {
	return s.Delete(id) // reutiliza la lógica existente
}

// AdminCountStats devuelve estadísticas globales de halcones.
func (s *HalconService) AdminCountStats() (map[string]int64, error) {
	total, err := facades.Orm().Query().Model(&models.Halcon{}).Count()
	if err != nil { return nil, err }

	active, err := facades.Orm().Query().Model(&models.Halcon{}).
		Where("is_active = ?", true).Count()
	if err != nil { return nil, err }

	assigned, err := facades.Orm().Query().Model(&models.HalconAssignment{}).
		Where("ended_at IS NULL").Count()
	if err != nil { return nil, err }

	return map[string]int64{
		"total":    total,
		"active":   active,
		"assigned": assigned,
	}, nil
}

// AdminGetAssignmentsHistory devuelve el historial de asignaciones de un halcón.
func (s *HalconService) AdminGetAssignmentsHistory(halconID uint) ([]models.HalconAssignment, error) {
	var assignments []models.HalconAssignment
	err := facades.Orm().Query().
		Model(&models.HalconAssignment{}).
		With("User").
		With("Moderator").
		Where("halcon_id = ?", halconID).
		Order("assigned_at DESC").
		Find(&assignments)
	return assignments, err
}


// HalconWithAssignment es un DTO que combina un halcón con su asignación activa.
type HalconWithAssignment struct {
	models.Halcon
	AssignedUser *models.User
	AssignedAt   *time.Time
}

// ListByModerator devuelve los halcones creados por un moderador (o todos si es admin),
// con su asignación activa resuelta.
func (s *HalconService) ListByModerator(
	moderatorID uint, page, limit int,
	search, active string, isAdmin bool,
) ([]HalconWithAssignment, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// ── Paso 1: si no es admin, resolvemos qué halcones ha creado este moderador ──
	var halconIDs []uint
	if !isAdmin {
		type row struct {
			HalconID uint `db:"halcon_id"`
		}
		var rows []row
		if err := facades.Orm().Query().
			Table("halcon_assignments").
			Select("DISTINCT halcon_id").
			Where("moderator_id = ?", moderatorID).
			Find(&rows); err != nil {
			return nil, 0, err
		}
		for _, r := range rows {
			halconIDs = append(halconIDs, r.HalconID)
		}
		if len(halconIDs) == 0 {
			return []HalconWithAssignment{}, 0, nil
		}
	}

	// ── Paso 2: query de halcones con filtros ──
	q := facades.Orm().Query().Model(&models.Halcon{})

	if !isAdmin {
		q = q.Where("id IN ?", halconIDs)
	}
	if search != "" {
		q = q.Where("name LIKE ?", "%"+search+"%")
	}
	if active == "1" {
		q = q.Where("is_active = ?", true)
	} else if active == "0" {
		q = q.Where("is_active = ?", false)
	}

	total, err := q.Count()
	if err != nil {
		return nil, 0, err
	}

	var halcones []models.Halcon
	if err := q.Order("id DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&halcones); err != nil {
		return nil, 0, err
	}

	// ── Paso 3: resolver usuario asignado por cada halcón ──
	result := make([]HalconWithAssignment, 0, len(halcones))
	for _, h := range halcones {
		item := HalconWithAssignment{Halcon: h}

		var a models.HalconAssignment
		if err := facades.Orm().Query().
			Where("halcon_id = ?", h.ID).
			Where("ended_at IS NULL").
			Order("assigned_at DESC").
			First(&a); err != nil {
			result = append(result, item)
			continue
		}

		var u models.User
		if err := facades.Orm().Query().
			Where("id = ?", a.UserID).
			First(&u); err == nil {
			item.AssignedUser = &u
			assignedAt := a.AssignedAt
			item.AssignedAt = &assignedAt
		}
		result = append(result, item)
	}

	return result, total, nil
}

// StatsByModerator devuelve estadísticas de halcones creados por un moderador.
func (s *HalconService) StatsByModerator(moderatorID uint, isAdmin bool) (map[string]int64, error) {
	stats := map[string]int64{
		"total":    0,
		"active":   0,
		"assigned": 0,
	}

	// Si no es admin, primero resolvemos los IDs de halcones que creó
	var halconIDs []uint
	if !isAdmin {
		type row struct {
			HalconID uint `db:"halcon_id"`
		}
		var rows []row
		if err := facades.Orm().Query().
			Table("halcon_assignments").
			Select("DISTINCT halcon_id").
			Where("moderator_id = ?", moderatorID).
			Find(&rows); err != nil {
			return stats, err
		}
		for _, r := range rows {
			halconIDs = append(halconIDs, r.HalconID)
		}
		if len(halconIDs) == 0 {
			return stats, nil
		}
	}

	// Total
	q := facades.Orm().Query().Model(&models.Halcon{})
	if !isAdmin {
		q = q.Where("id IN ?", halconIDs)
	}
	total, err := q.Count()
	if err != nil {
		return stats, err
	}
	stats["total"] = total

	// Activos
	q2 := facades.Orm().Query().Model(&models.Halcon{}).Where("is_active = ?", true)
	if !isAdmin {
		q2 = q2.Where("id IN ?", halconIDs)
	}
	active, err := q2.Count()
	if err != nil {
		return stats, err
	}
	stats["active"] = active

	// Asignados (con asignación activa)
	q3 := facades.Orm().Query().
		Model(&models.HalconAssignment{}).
		Where("ended_at IS NULL")
	if !isAdmin {
		q3 = q3.Where("moderator_id = ?", moderatorID)
	}
	assigned, err := q3.Count()
	if err != nil {
		return stats, err
	}
	stats["assigned"] = assigned

	return stats, nil
}
