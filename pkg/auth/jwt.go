package auth

import (
	"time"

	"github.com/DingAnZhong/feed/pkg/config"
	"github.com/golang-jwt/jwt/v5"
)

// Claims 自定义 JWT Claims
type Claims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

// GenerateToken 生成 JWT token
// 返回 access_token 和 refresh_token
func GenerateToken(userID int64) (accessToken, refreshToken string, err error) {
	authConf := config.Conf.Auth

	// 生成 access_token
	accessTokenClaims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(authConf.TokenTTLSeconds()) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "feed-system",
		},
	}

	accessTokenToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	accessTokenString, err := accessTokenToken.SignedString([]byte(authConf.JwtSecret))
	if err != nil {
		return "", "", err
	}

	// 生成 refresh_token
	refreshTokenClaims := &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(authConf.RefreshTokenTTLSeconds()) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "feed-system",
		},
	}

	refreshTokenToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshTokenClaims)
	refreshTokenString, err := refreshTokenToken.SignedString([]byte(authConf.JwtSecret))
	if err != nil {
		return "", "", err
	}

	return accessTokenString, refreshTokenString, nil
}

// ParseToken 解析 JWT token
func ParseToken(tokenString string) (*Claims, error) {
	authConf := config.Conf.Auth

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(authConf.JwtSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrSignatureInvalid
}

// IsTokenExpired 检查 token 是否过期
func IsTokenExpired(tokenString string) (bool, error) {
	claims, err := ParseToken(tokenString)
	if err != nil {
		return false, err
	}
	return time.Now().After(claims.ExpiresAt.Time), nil
}
