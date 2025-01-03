package tests_test

import "github.com/brianvoe/gofakeit/v6"

func genuser() (email, pass string) {
	email = gofakeit.Email()
	pass = gofakeit.Password(true, true, true, true, true, 8)
	return
}
