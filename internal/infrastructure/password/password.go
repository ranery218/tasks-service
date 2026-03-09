package password

import "golang.org/x/crypto/bcrypt"

func Hash(plain string, cost int) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), cost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

func Verify(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
