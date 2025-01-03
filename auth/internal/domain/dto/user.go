package dto

type CreateUser struct {
	LastName   *string
	FirstName  *string
	MiddleName *string
	Email      string
	Password   string
}
