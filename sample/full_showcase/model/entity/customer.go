package entity

import (
	"time"

	"github.com/jinzhu/copier"
	"gorm.io/gorm"

	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
)

// Customer is the customer entity (simplified for sample).
type Customer struct {
	ID       int64     `gorm:"primaryKey;column:id" json:"id"`
	Email    string    `gorm:"column:email;type:varchar(255);not null;uniqueIndex:idx_email" json:"email"`
	Password string    `gorm:"column:password;type:varchar(255);not null" json:"-"`
	Status   int       `gorm:"column:status;type:bigint;not null;default:0;index:idx_status" json:"status"`
	Created  time.Time `gorm:"column:created;type:datetime;not null;default:CURRENT_TIMESTAMP;index:idx_created" json:"created"`
	Modified time.Time `gorm:"column:modified;type:datetime;not null;default:CURRENT_TIMESTAMP;autoUpdateTime" json:"modified"`
}

// TableName returns the table name.
func (Customer) TableName() string {
	return "customers"
}

// BeforeCreate is the GORM before-create hook.
func (c *Customer) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	if c.Created.IsZero() {
		c.Created = now
	}
	if c.Modified.IsZero() {
		c.Modified = now
	}
	return nil
}

// BeforeUpdate is the GORM before-update hook.
func (c *Customer) BeforeUpdate(tx *gorm.DB) error {
	c.Modified = time.Now()
	return nil
}

// ToDto converts entity.Customer to dto.Customer.
func (c *Customer) ToDto() *dto.Customer {
	result := &dto.Customer{}
	copier.Copy(result, c)
	return result
}
