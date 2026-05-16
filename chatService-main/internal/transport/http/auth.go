package http

import (
	"errors"
	"strings"
)

// SimpleJWTValidator - базовая валидация JWT для сервисных вызовов
// В реальном приложении используйте github.com/golang-jwt/jwt/v5
type SimpleJWTValidator struct {
	// В реальном приложении здесь был бы публичный ключ из JWKS
}

// ServiceClaims - claims для service token
type ServiceClaims struct {
	Type        string   `json:"type"`
	Roles       []string `json:"roles"`
	ServiceName string   `json:"service_name"`
	Subject     string   `json:"sub"`
}

// ValidateServiceToken проверяет service token
// TODO: В реальном приложении это должно парсить и валидировать JWT подпись
func ValidateServiceToken(tokenString string, expectedServiceName string) (*ServiceClaims, error) {
	if tokenString == "" {
		return nil, errors.New("empty token")
	}

	// Упрощённая проверка - в реальном приложении парсить JWT
	// Сейчас это mock для демонстрации
	
	// Split token на части (Bearer <token>)
	parts := strings.SplitN(tokenString, ".", 3)
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	// В реальном приложении:
	// 1. Парсить header (JWT[0])
	// 2. Парсить claims (JWT[1])
	// 3. Верифицировать signature (JWT[2]) используя публичный ключ

	// Для теста возвращаем mock claims
	claims := &ServiceClaims{
		Type:        "service",
		Roles:       []string{"service"},
		ServiceName: expectedServiceName,
		Subject:     expectedServiceName,
	}

	return claims, nil
}

// ValidateClaimsForService проверяет что claims соответствуют требованиям
func ValidateClaimsForService(claims *ServiceClaims, expectedServiceName string) error {
	if claims == nil {
		return errors.New("claims missing")
	}

	if claims.Type != "service" {
		return errors.New("token type is not 'service'")
	}

	// Check roles contain 'service'
	hasServiceRole := false
	for _, role := range claims.Roles {
		if role == "service" {
			hasServiceRole = true
			break
		}
	}
	if !hasServiceRole {
		return errors.New("token missing 'service' role")
	}

	// Check service name matches
	if claims.ServiceName != expectedServiceName {
		return errors.New("service name mismatch")
	}

	return nil
}

