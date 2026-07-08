package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/ignorant05/control/api/cmd/model"
	"github.com/ignorant05/control/api/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	store     store.Store
	jwtSecret string
	jwtExpiry time.Duration
}

// NewAuthService creates an AuthService with configurable JWT settings
// jwtSecret and jwtExpiry come from config (environment variables)
func NewAuthService(s store.Store, jwtSecret string, jwtExpiry time.Duration) *AuthService {
	return &AuthService{
		store:     s,
		jwtSecret: jwtSecret,
		jwtExpiry: jwtExpiry,
	}
}

// HashPassword hashes a plaintext password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword verifies a password against its hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// generatePassword creates a cryptographically secure random password of the given length
// Length must be between 8 and 12 (enforced by caller)
func generatePassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			result[i] = charset[i%len(charset)]
			continue
		}
		result[i] = charset[n.Int64()]
	}
	return string(result)
}

// GenerateToken creates a JWT token for a user
func (s *AuthService) GenerateToken(user *model.User) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.jwtExpiry)
	claims := jwt.MapClaims{
		"user_id":   user.ID,
		"user_name": user.Name,
		"role":      string(user.Role),
		"project":   user.Project,
		"exp":       expiresAt.Unix(),
		"iat":       time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}

	return tokenString, expiresAt, nil
}

// ValidateToken parses and validates a JWT token
func (s *AuthService) ValidateToken(tokenString string) (*model.User, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	userName, _ := claims["user_name"].(string)
	roleStr, _ := claims["role"].(string)
	project, _ := claims["project"].(string)

	return &model.User{
		Name:    userName,
		Role:    model.Role(roleStr),
		Project: project,
	}, nil
}

// Login authenticates a user and returns a token
func (s *AuthService) Login(ctx context.Context, username, password string) (*model.LoginResponse, error) {
	user, err := s.store.GetUserByName(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	if !CheckPassword(password, user.Password) {
		return nil, fmt.Errorf("invalid credentials")
	}

	token, expiresAt, err := s.GenerateToken(user)
	if err != nil {
		return nil, err
	}

	user.LoggedIn = true
	if err := s.store.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	user.Password = ""
	return &model.LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      *user,
	}, nil
}

// RegisterApp creates a new project and admin user for an external application
// The admin password is auto-generated to a random length between 8 and 12 characters
func (s *AuthService) RegisterApp(ctx context.Context, appName string) (*model.AppRegistrationResponse, error) {
	project, err := s.store.CreateProject(ctx, appName)
	if err != nil {
		return nil, fmt.Errorf("create project: %w", err)
	}

	adminName := fmt.Sprintf("%s-admin", appName)
	passwordLen, _ := rand.Int(rand.Reader, big.NewInt(5))
	defaultPassword := generatePassword(8 + int(passwordLen.Int64()))

	hashedPw, err := HashPassword(defaultPassword)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	admin := &model.User{
		Name:     adminName,
		Role:     model.RoleAdmin,
		Project:  appName,
		Password: hashedPw,
	}
	if err := s.store.CreateUser(ctx, admin); err != nil {
		return nil, fmt.Errorf("create admin: %w", err)
	}

	return &model.AppRegistrationResponse{
		ProjectID:     project.ID,
		ProjectName:   project.Name,
		AdminUsername: adminName,
		AdminPassword: defaultPassword,
	}, nil
}
