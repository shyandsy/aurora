package dto

import (
	"time"

	commonModel "github.com/shyandsy/aurora/sample/full_showcase/common/model"
)

// Customer is the customer DTO (simplified).
type Customer struct {
	ID       int64     `json:"id"`
	Email    string    `json:"email"`
	Status   int       `json:"status"`
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
}

// GetCustomersReq is the get-customers request (with filters).
type GetCustomersReq struct {
	commonModel.PagingReq
	Email  string `form:"email"`
	Status *int   `form:"status"`
}

// CreateCustomerReq is the create-customer request.
type CreateCustomerReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// UpdateCustomerReq is the update-customer request.
type UpdateCustomerReq struct {
	Password *string `json:"password,omitempty"`
	Status   *int    `json:"status,omitempty"`
}
