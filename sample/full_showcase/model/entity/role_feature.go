package entity

import (
	"time"

	"github.com/jinzhu/copier"
	"gorm.io/gorm"

	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
)

// RoleFeature is the role-feature association entity.
type RoleFeature struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	RoleID    int64     `gorm:"column:role_id;type:bigint;not null;index:idx_role_id" json:"roleId"`
	FeatureID int64     `gorm:"column:feature_id;type:bigint;not null;index:idx_feature_id" json:"featureId"`
	Created   time.Time `gorm:"column:created;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"created"`
	Modified  time.Time `gorm:"column:modified;type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"modified"`

	Role    Role    `gorm:"foreignKey:RoleID;references:ID" json:"role,omitempty"`
	Feature Feature `gorm:"foreignKey:FeatureID;references:ID" json:"feature,omitempty"`
}

// TableName returns the table name.
func (RoleFeature) TableName() string {
	return "role_features"
}

// BeforeCreate is the GORM before-create hook.
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

// BeforeUpdate is the GORM before-update hook.
func (rf *RoleFeature) BeforeUpdate(tx *gorm.DB) error {
	rf.Modified = time.Now()
	return nil
}

// ToDto converts entity.RoleFeature to dto.RoleFeature.
func (rf *RoleFeature) ToDto() *dto.RoleFeature {
	result := &dto.RoleFeature{}
	copier.Copy(result, rf)
	return result
}
