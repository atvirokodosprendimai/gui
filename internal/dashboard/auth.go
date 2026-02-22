package dashboard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	roleAdmin = "admin"
	roleUser  = "user"
)

type User struct {
	ID           uint   `gorm:"primaryKey"`
	Email        string `gorm:"uniqueIndex"`
	PasswordHash string `gorm:"column:password_hash"`
	Role         string
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (User) TableName() string {
	return "users"
}

type Session struct {
	ID        uint      `gorm:"primaryKey"`
	Token     string    `gorm:"uniqueIndex"`
	UserID    uint      `gorm:"column:user_id"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (Session) TableName() string {
	return "sessions"
}

type userContextKey struct{}

func (a *App) BootstrapAdmin(email, password string) error {
	if a.db == nil {
		return nil
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || strings.TrimSpace(password) == "" {
		return nil
	}

	var count int64
	if err := a.db.Model(&User{}).Where("role = ?", roleAdmin).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return a.db.Create(&User{Email: email, PasswordHash: string(hash), Role: roleAdmin}).Error
}

func (a *App) CreateUser(email, password, role string) error {
	if a.db == nil {
		return nil
	}
	email = strings.ToLower(strings.TrimSpace(email))
	role = strings.ToLower(strings.TrimSpace(role))
	if email == "" || strings.TrimSpace(password) == "" {
		return errors.New("email and password are required")
	}
	if role != roleAdmin && role != roleUser {
		return errors.New("invalid role")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return a.db.Create(&User{Email: email, PasswordHash: string(hash), Role: role}).Error
}

func (a *App) lookupUserByEmail(email string) (*User, error) {
	if a.db == nil {
		return nil, gorm.ErrRecordNotFound
	}
	var u User
	err := a.db.Where("email = ?", strings.ToLower(strings.TrimSpace(email))).First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (a *App) authenticate(email, password string) (*User, error) {
	u, err := a.lookupUserByEmail(email)
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}
	return u, nil
}

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (a *App) createSession(userID uint) (string, error) {
	if a.db == nil {
		return "", nil
	}
	token, err := newSessionToken()
	if err != nil {
		return "", err
	}
	err = a.db.Create(&Session{
		Token:     token,
		UserID:    userID,
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}).Error
	if err != nil {
		return "", err
	}
	return token, nil
}

func (a *App) deleteSession(token string) {
	if a.db == nil || token == "" {
		return
	}
	_ = a.db.Where("token = ?", token).Delete(&Session{}).Error
	a.mu.Lock()
	delete(a.flashBySess, token)
	a.mu.Unlock()
}

func (a *App) userFromRequest(r *http.Request) (*User, error) {
	if a.db == nil {
		return &User{Email: "admin@local", Role: roleAdmin}, nil
	}
	c, err := r.Cookie("session_token")
	if err != nil {
		return nil, err
	}
	var s Session
	if err := a.db.Where("token = ? AND expires_at > ?", c.Value, time.Now().UTC()).First(&s).Error; err != nil {
		return nil, err
	}
	var u User
	if err := a.db.First(&u, s.UserID).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

func (a *App) cleanupExpiredSessions() {
	if a.db == nil {
		return
	}
	_ = a.db.Where("expires_at <= ?", time.Now().UTC()).Delete(&Session{}).Error
}

func (a *App) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := a.userFromRequest(r)
		if err != nil {
			if strings.HasPrefix(r.URL.Path, "/ui/") || strings.HasPrefix(r.URL.Path, "/any/") {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey{}, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func currentUser(r *http.Request) *User {
	u, _ := r.Context().Value(userContextKey{}).(*User)
	return u
}

func isAdmin(r *http.Request) bool {
	u := currentUser(r)
	return u != nil && u.Role == roleAdmin
}
