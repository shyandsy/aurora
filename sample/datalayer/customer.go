package datalayer

import (
	"context"
	"fmt"

	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/model/dto"
	"github.com/shyandsy/aurora/sample/model/entity"
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

// customerDatalayer 客户数据访问层
type customerDatalayer struct {
	DB *gorm.DB `inject:""`
}

// NewCustomerDatalayer 创建客户数据访问层
func NewCustomerDatalayer(app contracts.App) CustomerDatalayer {
	dl := &customerDatalayer{}
	if err := app.Resolve(dl); err != nil {
		panic(fmt.Errorf("failed to resolve CustomerDatalayer: %w", err))
	}
	return dl
}

// GetAll 获取所有客户（分页，支持筛选）
func (d *customerDatalayer) GetAll(ctx context.Context, req dto.GetCustomersReq) ([]entity.Customer, int64, error) {
	var customers []entity.Customer
	var total int64
	db := d.DB.WithContext(ctx).Model(&entity.Customer{})

	// 应用邮箱模糊匹配筛选
	if req.Email != "" {
		db = db.Where("email LIKE ?", "%"+req.Email+"%")
	}

	// 应用状态筛选
	if req.Status != nil {
		db = db.Where("status = ?", *req.Status)
	}

	// 获取总数
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 计算偏移量和限制
	offset := int((req.Page - 1) * req.PageSize)
	limit := int(req.PageSize)

	// 获取分页数据
	if err := db.Offset(offset).Limit(limit).Find(&customers).Error; err != nil {
		return nil, 0, err
	}

	return customers, total, nil
}

// GetByID 根据ID获取客户
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

// GetByEmail 根据邮箱获取客户
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

// Create 创建客户
func (d *customerDatalayer) Create(ctx context.Context, customer *entity.Customer) error {
	db := d.DB.WithContext(ctx)
	return db.Create(customer).Error
}

// Update 更新客户
// 注意：Email 字段不允许更新
func (d *customerDatalayer) Update(ctx context.Context, customer *entity.Customer) error {
	db := d.DB.WithContext(ctx)
	return db.Omit("email").Save(customer).Error
}

// Delete 删除客户
func (d *customerDatalayer) Delete(ctx context.Context, id int64) error {
	db := d.DB.WithContext(ctx)
	return db.Delete(&entity.Customer{}, id).Error
}
