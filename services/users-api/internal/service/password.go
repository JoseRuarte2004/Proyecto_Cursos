package service

import "golang.org/x/crypto/bcrypt"

type BcryptPasswordManager struct{}

func (BcryptPasswordManager) Hash(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hashed), nil
}

func (BcryptPasswordManager) Compare(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}
