package services

import (
	"errors"
	"github.com/goravel/framework/contracts/database/orm"
	"goravel/app/facades"
	"goravel/app/models"
)

// Serialize provisioning against the user row; the unique index is the final guard.
func (s *HalconService) EnsurePersonal(userID uint) (*models.Halcon, error) {
	var h models.Halcon
	err := facades.Orm().Transaction(func(tx orm.Query) error {
		var u models.User
		if err := tx.Where("id = ?", userID).LockForUpdate().FirstOrFail(&u); err != nil {
			return err
		}
		if !u.Status {
			return errors.New("usuario inactivo")
		}
		if err := tx.Where("owner_id = ?", userID).First(&h); err != nil {
			return err
		}
		if h.ID != 0 {
			return nil
		}
		token, err := generateToken()
		if err != nil {
			return err
		}
		h = models.Halcon{Name: "Halcón de " + u.Name, OwnerID: &userID, Token: token}
		return tx.Create(&h)
	})
	return &h, err
}

func (s *HalconService) SetRecipient(ownerID, recipientID uint) error {
	return facades.Orm().Transaction(func(tx orm.Query) error {
		var h models.Halcon
		if err := tx.Where("owner_id = ?", ownerID).LockForUpdate().FirstOrFail(&h); err != nil {
			return err
		}
		var recipient any
		if recipientID != 0 {
			if recipientID == ownerID {
				return errors.New("no puedes enviarte tu ubicación")
			}
			var u models.User
			if err := tx.Where("id = ?", recipientID).Where("status = ?", true).FirstOrFail(&u); err != nil {
				return errors.New("destinatario inválido o inactivo")
			}
			recipient = recipientID
		}
		_, err := tx.Model(&models.Halcon{}).Where("id = ?", h.ID).Update("recipient_id", recipient)
		return err
	})
}

// EXISTS avoids duplicate rows and ambiguous joined columns.
func (s *HalconService) VisibleTo(userID uint) ([]models.Halcon, error) {
	var u models.User
	if err := facades.Orm().Query().Where("id = ?", userID).Where("status = ?", true).FirstOrFail(&u); err != nil {
		return nil, err
	}
	q := facades.Orm().Query().Model(&models.Halcon{})
	if u.Role != "admin" {
		q = q.Where("owner_id = ? OR recipient_id = ? OR (moderator_id = ? AND ? = 'moderator') OR EXISTS (SELECT 1 FROM halcon_assignments a WHERE a.halcon_id = halcones.id AND a.user_id = ? AND a.ended_at IS NULL)", userID, userID, userID, u.Role, userID)
	}
	var hs []models.Halcon
	err := q.Order("id").Find(&hs)
	return hs, err
}

func (s *HalconService) CanManage(id, actorID uint) bool {
	h, err := s.GetByID(id)
	if err != nil || h.OwnerID != nil {
		return false
	}
	u, err := NewUserService().GetByID(actorID)
	return err == nil && u.Status && (u.Role == "admin" || (u.Role == "moderator" && h.ModeratorID == actorID))
}

// Check one coordinate's permission without loading every visible halcon.
// Status and role are read from the database for each event, including admins.
func (s *HalconService) CanView(id, userID uint) (bool, error) {
	count, err := facades.Orm().Query().Model(&models.Halcon{}).Where("id = ?", id).Where(`EXISTS (
		SELECT 1 FROM users u WHERE u.id = ? AND u.status = true AND (
			u.role = 'admin' OR halcones.owner_id = u.id OR halcones.recipient_id = u.id
			OR (u.role = 'moderator' AND halcones.moderator_id = u.id)
			OR EXISTS (SELECT 1 FROM halcon_assignments a WHERE a.halcon_id = halcones.id AND a.user_id = u.id AND a.ended_at IS NULL)
		)
	)`, userID).Count()
	return count > 0, err
}
