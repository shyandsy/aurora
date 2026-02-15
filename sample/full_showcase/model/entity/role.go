package entity

import (
	"time"

	"github.com/jinzhu/copier"
	"gorm.io/gorm"

	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
)

// Role is the role entity.
type Role struct {
	ID       int64     `gorm:"primaryKey;column:id" json:"id"`
	Name     string    `gorm:"column:name;type:varchar(255);not null;uniqueIndex:idx_name" json:"name"`
	Created  time.Time `gorm:"column:created;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"created"`
	Modified time.Time `gorm:"column:modified;type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"modified"`
}

// TableName returns the table name.
func (Role) TableName() string {
	return "roles"
}

// BeforeCreate is the GORM before-create hook.
func (r *Role) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if r.Created.IsZero() {
		r.Created = now
	}
	if r.Modified.IsZero() {
		r.Modified = now
	}
	return nil
}

// BeforeUpdate is the GORM before-update hook.
func (r *Role) BeforeUpdate(tx *gorm.DB) error {
	r.Modified = time.Now()
	return nil
}

// ToDto converts entity.Role to dto.Role.
func (r *Role) ToDto() *dto.Role {
	result := &dto.Role{}
	copier.Copy(result, r)
	return result
}
