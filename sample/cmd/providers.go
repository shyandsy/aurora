package main

import (
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/datalayer"
	serviceCustomer "github.com/shyandsy/aurora/sample/service/customer"
	serviceFeature "github.com/shyandsy/aurora/sample/service/feature"
	serviceRole "github.com/shyandsy/aurora/sample/service/role"
	serviceRoleFeature "github.com/shyandsy/aurora/sample/service/role_feature"
	serviceUser "github.com/shyandsy/aurora/sample/service/user"
)

// registerProviders 注册所有依赖注入的 providers
func registerProviders(app contracts.App) {
	// Datalayers
	app.ProvideAs(datalayer.NewUserDatalayer(app), (*datalayer.UserDatalayer)(nil))
	app.ProvideAs(datalayer.NewFeatureDatalayer(app), (*datalayer.FeatureDatalayer)(nil))
	app.ProvideAs(datalayer.NewRoleDatalayer(app), (*datalayer.RoleDatalayer)(nil))
	app.ProvideAs(datalayer.NewRoleFeatureDatalayer(app), (*datalayer.RoleFeatureDatalayer)(nil))
	app.ProvideAs(datalayer.NewCustomerDatalayer(app), (*datalayer.CustomerDatalayer)(nil))

	// Services
	app.ProvideAs(serviceUser.NewUserService(app), (*serviceUser.UserService)(nil))
	app.ProvideAs(serviceRole.NewRoleService(app), (*serviceRole.RoleService)(nil))
	app.ProvideAs(serviceFeature.NewFeatureService(app), (*serviceFeature.FeatureService)(nil))
	app.ProvideAs(serviceRoleFeature.NewRoleFeatureService(app), (*serviceRoleFeature.RoleFeatureService)(nil))
	app.ProvideAs(serviceCustomer.NewCustomerService(app), (*serviceCustomer.CustomerService)(nil))
}
