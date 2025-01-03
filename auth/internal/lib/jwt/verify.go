package jwt

import (
	"errors"
	"fmt"

	"mzhn/auth/internal/domain/entity"

	"github.com/golang-jwt/jwt/v5"
)

// Verify verifies a JSON Web Token (JWT) using a secret key.
//
// It takes two parameters: tokenString which is the JWT to be verified and secret which is the secret key used to sign the JWT.
//
// The function first attempts to parse the token using the jwt.ParseWithClaims function. It checks if the signing method used in the token is HMAC (Hash-based Message Authentication Code), and if it is, it returns the secret key for verification. If the signing method is not HMAC, it returns an error.
//
// If the token is successfully parsed, the function checks if the token has expired. If it has, it returns an error ErrTokenExpired. If the token has not expired, it attempts to parse the claims from the token. If the claims cannot be parsed, it returns an error.
//
// Finally, if the token is successfully verified and the claims are parsed, the function returns the user claims and nil error.
//
// Parameters:
//   - tokenString: the JWT to be verified
//   - secret: the secret key used to sign the JWT
//
// Returns:
//   - *entity.UserClaims: the user claims if the token is successfully verified and the claims are parsed
//   - error: an error if the token cannot be parsed or the claims cannot be parsed, or if the token has expired
func Verify(tokenString string, secret string) (*entity.UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithExpirationRequired())
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, err
	}

	claims, ok := token.Claims.(*claims)
	if !ok {
		return nil, errors.New("unable to parse claims")
	}

	return &claims.UserClaims, nil
}
