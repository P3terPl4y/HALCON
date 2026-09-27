package models

import (
	"time"

	"github.com/goravel/framework/database/orm"
)

// ============================================================
// User
// ============================================================
type User struct {
	orm.Model
	Name     string `gorm:"not null"`
	Email    string `gorm:"uniqueIndex;not null"`
	Phone    string `gorm:"uniqueIndex"`
	Password string `gorm:"not null"`
	Status   bool `gorm:"default:true"`
	Role     string `gorm:"default:'user'"` // user, moderator, admin
}

func (User) TableName() string { return "users" }

// ============================================================
// Halcon (dispositivo GPS)
// ============================================================
type Halcon struct {
	orm.Model
	Name     string     `gorm:"not null"`
	Token    string     `gorm:"uniqueIndex;not null"`
	IsActive bool       `gorm:"default:false"`
	LastLat  float64    `gorm:"type:decimal(10,7)"`
	LastLng  float64    `gorm:"type:decimal(10,7)"`
	LastSeen *time.Time `gorm:"nullable"`
}

func (Halcon) TableName() string { return "halcones" }

// ============================================================
// HalconAssignment (relación usuario ↔ halcón con historial)
// ============================================================
type HalconAssignment struct {
	orm.Model
	HalconID    uint       `gorm:"not null;index"`
	UserID      uint       `gorm:"not null;index"`
	ModeratorID uint       `gorm:"not null;index"`
	PackageID   string     `gorm:"nullable"`
	AssignedAt  time.Time  `gorm:"not null"`
	EndedAt     *time.Time `gorm:"nullable"`

	Halcon    *Halcon `gorm:"foreignKey:HalconID"`
	User      *User   `gorm:"foreignKey:UserID"`
	Moderator *User   `gorm:"foreignKey:ModeratorID"`
}

func (HalconAssignment) TableName() string { return "halcon_assignments" }
