package requests

type CreateUserRequest struct {
	Name     string `json:"name"     validate:"required,min=2"`
	Email    string `json:"email"    validate:"required,email"`
	Phone    string `json:"phone"    validate:"required"`
	Password string `json:"password" validate:"required,min=8"`
	Role     string `json:"role"     validate:"required,oneof=user moderator admin"`
}

type UserUpdateRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}


type CreateHalconRequest struct {
	ModeratorID uint `json:"moderator_id"`
	Name string `json:"name" validate:"required,min=2,max=100"`
}

type UpdateHalconRequest struct {
	Name string `json:"name" validate:"required,min=2,max=100"`
}

type AssignHalconRequest struct {
	HalconID  uint   `json:"halcon_id" validate:"required"`
	UserID    uint   `json:"user_id"   validate:"required"`
	PackageID string `json:"package_id"`
}
