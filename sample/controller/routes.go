package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/common/middleware"
	"github.com/shyandsy/aurora/sample/controller/auth"
	"github.com/shyandsy/aurora/sample/controller/customer"
	"github.com/shyandsy/aurora/sample/controller/feature"
	"github.com/shyandsy/aurora/sample/controller/role"
	"github.com/shyandsy/aurora/sample/controller/role_feature"
	"github.com/shyandsy/aurora/sample/controller/user"
)

// GetRoutes 返回管理员服务的所有路由
// app parameter is used to create JWT middleware for protected routes
func GetRoutes(app contracts.App) []contracts.Route {
	serviceName := app.Name()
	apiPrefix := "/api/" + serviceName + "/v1"

	return []contracts.Route{
		// Auth routes (no JWT required)
		{
			Method:  "POST",
			Path:    apiPrefix + "/auth/login",
			Handler: auth.Login,
		},
		// User routes (JWT required with feature check)
		{
			Method:      "GET",
			Path:        apiPrefix + "/user",
			Handler:     user.GetUsers,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "user.get")},
		},
		{
			Method:      "GET",
			Path:        apiPrefix + "/user/:id",
			Handler:     user.GetUser,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "user.get")},
		},
		{
			Method:      "POST",
			Path:        apiPrefix + "/user",
			Handler:     user.CreateUser,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "user.create")},
		},
		{
			Method:      "PUT",
			Path:        apiPrefix + "/user/:id",
			Handler:     user.UpdateUser,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "user.update")},
		},
		{
			Method:      "DELETE",
			Path:        apiPrefix + "/user/:id",
			Handler:     user.DeleteUser,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "user.delete")},
		},
		// Role routes (JWT required with feature check)
		{
			Method:      "GET",
			Path:        apiPrefix + "/role",
			Handler:     role.GetRoles,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "role.get")},
		},
		{
			Method:      "GET",
			Path:        apiPrefix + "/role/:id",
			Handler:     role.GetRole,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "role.get")},
		},
		{
			Method:      "POST",
			Path:        apiPrefix + "/role",
			Handler:     role.CreateRole,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "role.create")},
		},
		{
			Method:      "PUT",
			Path:        apiPrefix + "/role/:id",
			Handler:     role.UpdateRole,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "role.update")},
		},
		{
			Method:      "DELETE",
			Path:        apiPrefix + "/role/:id",
			Handler:     role.DeleteRole,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "role.delete")},
		},
		// Feature routes (JWT required with feature check)
		{
			Method:      "GET",
			Path:        apiPrefix + "/feature",
			Handler:     feature.GetFeatures,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "feature.get")},
		},
		{
			Method:      "GET",
			Path:        apiPrefix + "/feature/:id",
			Handler:     feature.GetFeature,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "feature.get")},
		},
		// RoleFeature routes (JWT required with feature check)
		{
			Method:      "GET",
			Path:        apiPrefix + "/role-feature",
			Handler:     role_feature.GetRoleFeatures,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "rolefeature.get")},
		},
		{
			Method:      "GET",
			Path:        apiPrefix + "/role-feature/:id",
			Handler:     role_feature.GetRoleFeature,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "rolefeature.get")},
		},
		{
			Method:      "POST",
			Path:        apiPrefix + "/role-feature",
			Handler:     role_feature.CreateRoleFeature,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "rolefeature.create")},
		},
		{
			Method:      "DELETE",
			Path:        apiPrefix + "/role-feature/:id",
			Handler:     role_feature.DeleteRoleFeature,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "rolefeature.delete")},
		},
		// Customer routes (JWT required with feature check)
		{
			Method:      "GET",
			Path:        apiPrefix + "/customer",
			Handler:     customer.GetCustomers,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "customer.get")},
		},
		{
			Method:      "GET",
			Path:        apiPrefix + "/customer/:id",
			Handler:     customer.GetCustomer,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "customer.get")},
		},
		{
			Method:      "POST",
			Path:        apiPrefix + "/customer",
			Handler:     customer.CreateCustomer,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "customer.create")},
		},
		{
			Method:      "PUT",
			Path:        apiPrefix + "/customer/:id",
			Handler:     customer.UpdateCustomer,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "customer.update")},
		},
		{
			Method:      "DELETE",
			Path:        apiPrefix + "/customer/:id",
			Handler:     customer.DeleteCustomer,
			Middlewares: []gin.HandlerFunc{middleware.JWTAuthMiddleware(app, "customer.delete")},
		},
	}
}
