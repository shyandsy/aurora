package entity

import (
	"time"

	"github.com/jinzhu/copier"
	"gorm.io/gorm"

	"github.com/shyandsy/aurora/sample/model/dto"
)

// RoleFeature 角色功能关联实体
type RoleFeature struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	RoleID    int64     `gorm:"column:role_id;type:bigint;not null;index:idx_role_id" json:"roleId"`
	FeatureID int64     `gorm:"column:feature_id;type:bigint;not null;index:idx_feature_id" json:"featureId"`
	Created   time.Time `gorm:"column:created;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"created"`
	Modified  time.Time `gorm:"column:modified;type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"modified"`

	// 关联对象
	Role    Role    `gorm:"foreignKey:RoleID;references:ID" json:"role,omitempty"`
	Feature Feature `gorm:"foreignKey:FeatureID;references:ID" json:"feature,omitempty"`
}

// TableName 指定表名
func (RoleFeature) TableName() string {
	return "role_features"
}

// BeforeCreate 创建前钩子
func (rf *RoleFeature) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if rf.Created.IsZero() {
		rf.Created = now
	}
	if rf.Modified.IsZero() {
		rf.Modified = now
	}
	return nil
}

// BeforeUpdate 更新前钩子
func (rf *RoleFeature) BeforeUpdate(tx *gorm.DB) error {
	rf.Modified = time.Now()
	return nil
}

// ToDto 将 entity.RoleFeature 转换为 dto.RoleFeature
func (rf *RoleFeature) ToDto() *dto.RoleFeature {
	result := &dto.RoleFeature{}
	copier.Copy(result, rf)
	return result
}
