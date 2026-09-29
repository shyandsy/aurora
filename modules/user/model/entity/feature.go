package entity

import (
	"time"

	"github.com/jinzhu/copier"
	"gorm.io/gorm"

	"github.com/shyandsy/aurora/modules/user/model/dto"
)

// Feature 功能实体
type Feature struct {
	ID   int64  `gorm:"primaryKey;column:id" json:"id"`
	Name string `gorm:"column:name;type:varchar(255);not null;uniqueIndex:idx_name" json:"name"`
	// Module/Action/Kind 是 RBAC 分档维度列。不参与鉴权(鉴权只认 name),供前端筛选/分组/读写分档展示,
	// 以及签发时的 UI 富展开语义(ui.page.* / ui.menu.* 入口标记)。
	//   kind:   api(业务接口 feature) / ui(ui.menu.* / ui.page.* / ui.component.*)
	//   action: read / write(api),view / operate(ui.page.*)
	//   module: 归属页/资源,如 customer、commission
	Module   string    `gorm:"column:module;type:varchar(64);not null;default:''" json:"module"`
	Action   string    `gorm:"column:action;type:varchar(16);not null;default:''" json:"action"`
	Kind     string    `gorm:"column:kind;type:varchar(16);not null;default:'api'" json:"kind"`
	Created  time.Time `gorm:"column:created;type:datetime;not null;default:CURRENT_TIMESTAMP" json:"created"`
	Modified time.Time `gorm:"column:modified;type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"modified"`
}

// TableName 指定表名
func (Feature) TableName() string {
	return "user_features"
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
