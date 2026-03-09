package jwt

import (
	"errors"
	"fmt"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

type Service struct {
	accessSecret []byte
}

type AccessClaims struct {
	UserID  int64 `json:"user_id"`
	IsAdmin bool  `json:"is_admin"`
	jwtv5.RegisteredClaims
}

func NewService(accessSecret string) *Service {
	return &Service{
		accessSecret: []byte(accessSecret),
	}
}

func (s *Service) GenerateAccessToken(userID int64, isAdmin bool, ttl time.Duration) (string, error) {
	claims := AccessClaims{
		UserID:  userID,
		IsAdmin: isAdmin,
		RegisteredClaims: jwtv5.RegisteredClaims{
			ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwtv5.NewNumericDate(time.Now()),
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	token := jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims)
	return token.SignedString(s.accessSecret)
}

func (s *Service) ParseAccessToken(tokenString string) (AccessClaims, error) {
	var claims AccessClaims

	token, err := jwtv5.ParseWithClaims(tokenString, &claims, func(token *jwtv5.Token) (any, error) {
		if _, ok := token.Method.(*jwtv5.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.accessSecret, nil
	})
	if err != nil || !token.Valid {
		return AccessClaims{}, ErrInvalidToken
	}

	if claims.UserID == 0 {
		return AccessClaims{}, ErrInvalidToken
	}

	return claims, nil
}
