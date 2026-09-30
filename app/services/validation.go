package services

import (
	"errors"
	"math"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"goravel/app/requests"
)

func ValidRole(role string) bool { return role == "user" || role == "moderator" || role == "admin" }

func ValidateName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if len([]rune(name)) < 2 || len(name) > 100 {
		return "", errors.New("el nombre debe tener de 2 a 100 caracteres")
	}
	return name, nil
}

func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	a, err := mail.ParseAddress(email)
	if err != nil || a.Address != email || len(email) > 254 {
		return "", errors.New("correo inválido")
	}
	return email, nil
}

func ValidatePassword(password string) error {
	// bcrypt limits bytes, not Unicode code points.
	if len(password) < 8 || len(password) > 72 {
		return errors.New("la contraseña debe tener de 8 a 72 bytes")
	}
	return nil
}

func NormalizeCreateUser(req *requests.CreateUserRequest) error {
	if req == nil {
		return errors.New("datos de usuario obligatorios")
	}
	var err error
	if req.Name, err = ValidateName(req.Name); err != nil {
		return err
	}
	if req.Email, err = NormalizeEmail(req.Email); err != nil {
		return err
	}
	req.Phone = strings.TrimSpace(req.Phone)
	if req.Phone == "" || len(req.Phone) > 40 {
		return errors.New("teléfono obligatorio de hasta 40 caracteres")
	}
	if !ValidRole(req.Role) {
		return errors.New("rol inválido")
	}
	return ValidatePassword(req.Password)
}

// Shared by self-service and administration; self-service passes an empty role.
func PrepareUserUpdates(req requests.UserUpdateRequest, role string) (map[string]any, error) {
	updates := make(map[string]any)
	if req.Name != "" {
		name, err := ValidateName(req.Name)
		if err != nil {
			return nil, err
		}
		updates["name"] = name
	}
	if req.Email != "" {
		email, err := NormalizeEmail(req.Email)
		if err != nil {
			return nil, err
		}
		updates["email"] = email
	}
	if req.Password != "" {
		if err := ValidatePassword(req.Password); err != nil {
			return nil, err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		updates["password"] = string(hash)
	}
	if role != "" {
		if !ValidRole(role) {
			return nil, errors.New("rol inválido")
		}
		updates["role"] = role
	}
	return updates, nil
}

func ValidCoordinates(lat, lng float64) bool {
	return !math.IsNaN(lat) && !math.IsInf(lat, 0) && !math.IsNaN(lng) && !math.IsInf(lng, 0) && lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}
