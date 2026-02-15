package datalayer

import (
	"context"
	"fmt"

	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/model/dto"
	"github.com/shyandsy/aurora/sample/full_showcase/model/entity"
	"gorm.io/gorm"
)

type CustomerDatalayer interface {
	GetAll(ctx context.Context, req dto.GetCustomersReq) ([]entity.Customer, int64, error)
	GetByID(ctx context.Context, id int64) (*entity.Customer, error)
	GetByEmail(ctx context.Context, email string) (*entity.Customer, error)
	Create(ctx context.Context, customer *entity.Customer) error
	Update(ctx context.Context, customer *entity.Customer) error
	Delete(ctx context.Context, id int64) error
}

// customerDatalayer is the customer data access layer.
type customerDatalayer struct {
	DB *gorm.DB `inject:""`
}

// NewCustomerDatalayer creates the customer datalayer.
func NewCustomerDatalayer(app contracts.App) CustomerDatalayer {
	dl := &customerDatalayer{}
	if err := app.Resolve(dl); err != nil {
		panic(fmt.Errorf("failed to resolve CustomerDatalayer: %w", err))
	}
	return dl
}

// GetAll gets all customers (paged, filterable).
func (d *customerDatalayer) GetAll(ctx context.Context, req dto.GetCustomersReq) ([]entity.Customer, int64, error) {
	var customers []entity.Customer
	var total int64
	db := d.DB.WithContext(ctx).Model(&entity.Customer{})

	// Apply email filter
	if req.Email != "" {
		db = db.Where("email LIKE ?", "%"+req.Email+"%")
	}

	// Apply status filter
	if req.Status != nil {
		db = db.Where("status = ?", *req.Status)
	}

	// Get total count
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Calc offset and limit
	offset := int((req.Page - 1) * req.PageSize)
	limit := int(req.PageSize)

	// Get paged data
	if err := db.Offset(offset).Limit(limit).Find(&customers).Error; err != nil {
		return nil, 0, err
	}

	return customers, total, nil
}

// GetByID gets customer by ID.
func (d *customerDatalayer) GetByID(ctx context.Context, id int64) (*entity.Customer, error) {
	var customer entity.Customer
	db := d.DB.WithContext(ctx)
	if err := db.First(&customer, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &customer, nil
}

// GetByEmail gets customer by email.
func (d *customerDatalayer) GetByEmail(ctx context.Context, email string) (*entity.Customer, error) {
	var customer entity.Customer
	db := d.DB.WithContext(ctx)
	if err := db.Where("email = ?", email).First(&customer).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &customer, nil
}

// Create creates a customer.
func (d *customerDatalayer) Create(ctx context.Context, customer *entity.Customer) error {
	db := d.DB.WithContext(ctx)
	return db.Create(customer).Error
}

// Update updates a customer.
// Email is not updatable
func (d *customerDatalayer) Update(ctx context.Context, customer *entity.Customer) error {
	db := d.DB.WithContext(ctx)
	return db.Omit("email").Save(customer).Error
}

// Delete deletes a customer.
func (d *customerDatalayer) Delete(ctx context.Context, id int64) error {
	db := d.DB.WithContext(ctx)
	return db.Delete(&entity.Customer{}, id).Error
}
