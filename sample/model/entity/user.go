package entity

import (
	"time"

	"github.com/jinzhu/copier"
	"gorm.io/gorm"

	"github.com/shyandsy/aurora/sample/model/dto"
)

// User 管理员用户实体
type User struct {
	ID       int64     `gorm:"primaryKey;column:id" json:"id"`
	Email    string    `gorm:"column:email;type:varchar(255);not null;uniqueIndex:idx_email" json:"email"`
	Password string    `gorm:"column:password;type:varchar(255);not null" json:"-"`
	RoleID   int64     `gorm:"column:role_id;type:bigint;not null;index:idx_role_id" json:"roleId"`
	Status   int       `gorm:"column:status;type:bigint;not null;default:0;index:idx_status" json:"status"`
	Created  time.Time `gorm:"column:created;type:datetime;not null;default:CURRENT_TIMESTAMP;index:idx_created" json:"created"`
	Modified time.Time `gorm:"column:modified;type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"modified"`

	// 关联对象
	Role Role `gorm:"foreignKey:RoleID;references:ID" json:"role,omitempty"`
}

// TableName 指定表名
func (User) TableName() string {
	return "users"
}

// BeforeCreate 创建前钩子
func (u *User) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if u.Created.IsZero() {
		u.Created = now
	}
	if u.Modified.IsZero() {
		u.Modified = now
	}
	return nil
}

// BeforeUpdate 更新前钩子
func (u *User) BeforeUpdate(tx *gorm.DB) error {
	u.Modified = time.Now()
	return nil
}

// ToDto 将 entity.User 转换为 dto.User
func (u *User) ToDto() *dto.User {
	result := &dto.User{}
	copier.Copy(result, u)

	// 转换 Role
	if u.Role.ID > 0 {
		result.Role = u.Role.ToDto()
	}

	return result
}
