package auth

import (
	"api/app/models/user"
	"errors"
)

func Attempt(email string, password string) (user.User, error) {
	userModel := user.GetByMulti(email)

	if userModel.ID == 0 {
		return user.User{}, errors.New("login id is not exist")
	}

	if !userModel.ComparePassword(password) {
		return user.User{}, errors.New("password is incorrecte")
	}
	return userModel, nil
}
