package main

import (
	"github.com/shyandsy/aurora/contracts"
	"github.com/shyandsy/aurora/sample/full_showcase/datalayer"
	serviceCustomer "github.com/shyandsy/aurora/sample/full_showcase/service/customer"
	serviceFeature "github.com/shyandsy/aurora/sample/full_showcase/service/feature"
	serviceRole "github.com/shyandsy/aurora/sample/full_showcase/service/role"
	serviceRoleFeature "github.com/shyandsy/aurora/sample/full_showcase/service/role_feature"
	serviceUser "github.com/shyandsy/aurora/sample/full_showcase/service/user"
)

// registerProviders registers all DI providers.
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
