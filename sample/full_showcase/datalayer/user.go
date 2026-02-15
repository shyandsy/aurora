package datalayer

import (
	"context"
	"fmt"

	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/entity"
	"gorm.io/gorm"
)

type UserDatalayer interface {
	GetAll(ctx context.Context, offset, limit int) ([]entity.User, int64, error)
	GetByID(ctx context.Context, id int64) (*entity.User, error)
	GetByEmail(ctx context.Context, email string) (*entity.User, error)
	Create(ctx context.Context, user *entity.User) error
	Update(ctx context.Context, user *entity.User) error
	Delete(ctx context.Context, id int64) error
}

// userDatalayer is the user data access layer.
type userDatalayer struct {
	DB *gorm.DB `inject:""`
}

// NewUserDatalayer creates the user datalayer.
func NewUserDatalayer(app contracts.App) UserDatalayer {
	dl := &userDatalayer{}
	if err := app.Resolve(dl); err != nil {
		panic(fmt.Errorf("failed to resolve UserDatalayer: %w", err))
	}
	return dl
}

// GetAll gets all users (paged).
func (d *userDatalayer) GetAll(ctx context.Context, offset, limit int) ([]entity.User, int64, error) {
	var users []entity.User
	var total int64
	db := d.DB.WithContext(ctx)

	// Get total count
	if err := db.Model(&entity.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Get paged data, preload Role
	if err := db.Preload("Role").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// GetByID gets user by ID.
func (d *userDatalayer) GetByID(ctx context.Context, id int64) (*entity.User, error) {
	var user entity.User
	db := d.DB.WithContext(ctx)
	if err := db.Preload("Role").First(&user, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// GetByEmail gets user by email.
func (d *userDatalayer) GetByEmail(ctx context.Context, email string) (*entity.User, error) {
	var user entity.User
	db := d.DB.WithContext(ctx)
	if err := db.Preload("Role").Where("email = ?", email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// Create creates a user.
func (d *userDatalayer) Create(ctx context.Context, user *entity.User) error {
	db := d.DB.WithContext(ctx)
	return db.Create(user).Error
}

// Update updates a user.
func (d *userDatalayer) Update(ctx context.Context, user *entity.User) error {
	db := d.DB.WithContext(ctx)
	return db.Save(user).Error
}

// Delete deletes a user.
func (d *userDatalayer) Delete(ctx context.Context, id int64) error {
	db := d.DB.WithContext(ctx)
	return db.Delete(&entity.User{}, id).Error
}
