package feature

type UserInfo interface {
	GetID() int64
	GetEmail() string
	GetFirstname() string
	GetLastname() string
}

type UserService interface {
	GetUserByID(userID int64) (UserInfo, error)
}

// func ProvideUserService(customerService CustomerService) UserService {
// 	return customerService
// }
