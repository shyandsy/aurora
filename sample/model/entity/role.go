package entity

import (
	"time"

	"github.com/jinzhu/copier"
	"gorm.io/gorm"

	"github.com/shyandsy/aurora/sample/model/dto"
)

// Role 角色实体
type Role struct {
	ID       int64     `gorm:"primaryKey;column:id" json:"id"`
	Name     string    `gorm:"column:name;type:varchar(255);not null;uniqueIndex:idx_name" json:"name"`
	Created  time.Time `gorm:"column:created;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"created"`
	Modified time.Time `gorm:"column:modified;type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"modified"`
}

// TableName 指定表名
func (Role) TableName() string {
	return "roles"
}

// BeforeCreate 创建前钩子
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

// BeforeUpdate 更新前钩子
func (r *Role) BeforeUpdate(tx *gorm.DB) error {
	r.Modified = time.Now()
	return nil
}

// ToDto 将 entity.Role 转换为 dto.Role
func (r *Role) ToDto() *dto.Role {
	result := &dto.Role{}
	copier.Copy(result, r)
	return result
}
