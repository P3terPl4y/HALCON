package services

import (
	"errors"
	"fmt"
	"goravel/app/facades"
	"goravel/app/models"
	"goravel/app/requests"
	"log"
	"strings"

	"github.com/goravel/framework/contracts/database/orm"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct{}

func NewUserService() *UserService { return &UserService{} }

// GetAll devuelve usuarios paginados y el total.
func (s *UserService) GetAll(page, limit int) ([]models.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var users []models.User
	if err := facades.Orm().Query().Model(&models.User{}).
		Order("id DESC").Offset(offset).Limit(limit).Find(&users); err != nil {
		return nil, 0, err
	}

	total, err := facades.Orm().Query().Model(&models.User{}).Count()
	if err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// GetByID obtiene un usuario por ID.
func (s *UserService) GetByID(id uint) (*models.User, error) {
	var user models.User
	if err := facades.Orm().Query().Where("id = ?", id).FirstOrFail(&user); err != nil {
		return nil, errors.New("usuario no encontrado")
	}
	return &user, nil
}

// CreateUserWithRole crea un usuario con el rol indicado.
// Ya no crea perfiles de Driver/Client: el rol vive directamente en User.
func (s *UserService) CreateUserWithRole(req *requests.CreateUserRequest) (*models.User, error) {
	if err := NormalizeCreateUser(req); err != nil {
		return nil, err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := models.User{
		Name:     req.Name,
		Email:    req.Email,
		Phone:    req.Phone,
		Password: string(hashed),
		Status:   true,
		Role:     req.Role,
	}

	if err := facades.Orm().Query().Create(&user); err != nil {
		log.Printf("Error al crear usuario: %v", err)
		return nil, fmt.Errorf("crear usuario: %w", err)
	}
	return &user, nil
}

// Update aplica cambios parciales al usuario.
func (s *UserService) Update(id uint, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	_, err := facades.Orm().Query().Model(&models.User{}).
		Where("id = ?", id).Update(updates)
	return err
}

// Delete elimina un usuario por ID.
func (s *UserService) Delete(id uint) error {
	return s.AdminDeleteUser(id)
}

// Paginated search keeps users beyond the first dropdown page assignable.
func (s *UserService) SearchAssignable(page, limit int, search string) ([]models.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	search = strings.TrimSpace(search)
	if len(search) > 254 {
		return nil, 0, errors.New("búsqueda demasiado larga")
	}
	q := facades.Orm().Query().Model(&models.User{}).Where("status = ?", true).Where("role = ?", "user")
	if search != "" {
		q = q.Where("name ILIKE ? OR email ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	var users []models.User
	var total int64
	err := q.Order("id").Paginate(page, limit, &users, &total)
	return users, total, err
}

// HasRole verifica si un usuario tiene alguno de los roles dados.
func (s *UserService) HasRole(userID uint, roles ...string) (bool, error) {
	user, err := s.GetByID(userID)
	if err != nil {
		return false, err
	}
	for _, r := range roles {
		if user.Role == r {
			return true, nil
		}
	}
	return false, nil
}

// ============================================================
// ADMIN — CRUD de usuarios
// ============================================================

// AdminListUsers devuelve usuarios paginados con búsqueda opcional.
func (s *UserService) AdminListUsers(page, limit int, search, role string) ([]models.User, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	query := facades.Orm().Query().Model(&models.User{})

	if search != "" {
		like := "%" + search + "%"
		query = query.Where("name LIKE ? OR email LIKE ?", like, like)
	}
	if role != "" {
		query = query.Where("role = ?", role)
	}

	var users []models.User
	var total int64
	if err := query.Paginate(page, limit, &users, &total); err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// AdminGetUser obtiene un usuario por ID (sin restricción de sesión).
func (s *UserService) AdminGetUser(id uint) (*models.User, error) {
	var user models.User
	if err := facades.Orm().Query().Where("id = ?", id).FirstOrFail(&user); err != nil {
		return nil, errors.New("usuario no encontrado")
	}
	return &user, nil
}

// AdminUpdateUser actualiza campos de un usuario desde el panel admin.
func (s *UserService) AdminUpdateUser(id uint, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	_, err := facades.Orm().Query().Model(&models.User{}).
		Where("id = ?", id).Update(updates)
	return err
}

// AdminDeleteUser elimina un usuario y todas sus asignaciones de halcones.
func (s *UserService) AdminDeleteUser(id uint) error {
	return facades.Orm().Transaction(func(tx orm.Query) error {
		if _, err := tx.Where("user_id = ?", id).Delete(&models.HalconAssignment{}); err != nil {
			return err
		}
		if _, err := tx.Model(&models.Halcon{}).Where("moderator_id = ?", id).Update("moderator_id", 0); err != nil {
			return err
		}
		if _, err := tx.Model(&models.HalconAssignment{}).Where("moderator_id = ?", id).Update("moderator_id", 0); err != nil {
			return err
		}
		if _, err := tx.Where("id = ?", id).Delete(&models.User{}); err != nil {
			return err
		}
		return nil
	})
}

// AdminCreateUser crea un usuario desde el panel admin (mismo flujo que register).
func (s *UserService) AdminCreateUser(req *requests.CreateUserRequest) (*models.User, error) {
	return s.CreateUserWithRole(req)
}

// AdminCountByRole devuelve el número de usuarios por rol.
func (s *UserService) AdminCountByRole() (map[string]int64, error) {
	result := map[string]int64{}
	for _, role := range []string{"user", "moderator", "admin"} {
		count, err := facades.Orm().Query().Model(&models.User{}).
			Where("role = ?", role).Count()
		if err != nil {
			return nil, err
		}
		result[role] = count
	}
	return result, nil
}
