package entity

import (
	"time"

	"github.com/jinzhu/copier"
	"gorm.io/gorm"

	"github.com/shyandsy/aurora/sample/model/dto"
)

// Feature 功能实体
type Feature struct {
	ID       int64     `gorm:"primaryKey;column:id" json:"id"`
	Name     string    `gorm:"column:name;type:varchar(255);not null;uniqueIndex:idx_name" json:"name"`
	Created  time.Time `gorm:"column:created;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"created"`
	Modified time.Time `gorm:"column:modified;type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"modified"`
}

// TableName 指定表名
func (Feature) TableName() string {
	return "features"
}

// BeforeCreate 创建前钩子
func (f *Feature) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if f.Created.IsZero() {
		f.Created = now
	}
	if f.Modified.IsZero() {
		f.Modified = now
	}
	return nil
}

// BeforeUpdate 更新前钩子
func (f *Feature) BeforeUpdate(tx *gorm.DB) error {
	f.Modified = time.Now()
	return nil
}

// ToDto 将 entity.Feature 转换为 dto.Feature
func (f *Feature) ToDto() *dto.Feature {
	result := &dto.Feature{}
	copier.Copy(result, f)
	return result
}
