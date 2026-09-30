package services

import (
	"encoding/hex"
	"math"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"goravel/app/requests"
)

func TestUserCreationValidation(t *testing.T) {
	cases := []struct {
		name   string
		change func(*requests.CreateUserRequest)
		valid  bool
	}{
		{"valid", func(*requests.CreateUserRequest) {}, true},
		{"trim and lowercase", func(r *requests.CreateUserRequest) {
			r.Name = "  Ana  "
			r.Email = " ANA@EXAMPLE.TEST "
			r.Phone = " 123 "
		}, true},
		{"short name", func(r *requests.CreateUserRequest) { r.Name = "A" }, false},
		{"long name", func(r *requests.CreateUserRequest) { r.Name = strings.Repeat("a", 101) }, false},
		{"invalid email", func(r *requests.CreateUserRequest) { r.Email = "' OR 1=1 --" }, false},
		{"display name email", func(r *requests.CreateUserRequest) { r.Email = "Ana <ana@example.test>" }, false},
		{"empty phone", func(r *requests.CreateUserRequest) { r.Phone = " " }, false},
		{"long phone", func(r *requests.CreateUserRequest) { r.Phone = strings.Repeat("1", 41) }, false},
		{"short password", func(r *requests.CreateUserRequest) { r.Password = "short" }, false},
		{"bcrypt limit", func(r *requests.CreateUserRequest) { r.Password = strings.Repeat("a", 72) }, true},
		{"bcrypt overflow", func(r *requests.CreateUserRequest) { r.Password = strings.Repeat("a", 73) }, false},
		{"unicode overflow", func(r *requests.CreateUserRequest) { r.Password = strings.Repeat("á", 37) }, false},
		{"invalid role", func(r *requests.CreateUserRequest) { r.Role = "superadmin" }, false},
		{"moderator", func(r *requests.CreateUserRequest) { r.Role = "moderator" }, true},
		{"admin", func(r *requests.CreateUserRequest) { r.Role = "admin" }, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := requests.CreateUserRequest{Name: "Ana", Email: "ana@example.test", Phone: "123", Password: "Password123!", Role: "user"}
			c.change(&r)
			err := NormalizeCreateUser(&r)
			if (err == nil) != c.valid {
				t.Fatalf("valid=%v error=%v", c.valid, err)
			}
			if c.name == "trim and lowercase" && (r.Name != "Ana" || r.Email != "ana@example.test" || r.Phone != "123") {
				t.Fatal("normalization failed")
			}
		})
	}
	if NormalizeCreateUser(nil) == nil {
		t.Fatal("nil request accepted")
	}
}

func TestProfileAndAdminUpdates(t *testing.T) {
	updates, err := PrepareUserUpdates(requests.UserUpdateRequest{}, "")
	if err != nil || len(updates) != 0 {
		t.Fatalf("empty patch: %v %v", updates, err)
	}
	for _, r := range []requests.UserUpdateRequest{{Name: " "}, {Name: strings.Repeat("a", 101)}, {Email: "invalid"}, {Password: "x"}, {Password: strings.Repeat("x", 73)}} {
		if _, err := PrepareUserUpdates(r, ""); err == nil {
			t.Fatalf("invalid patch accepted: %+v", r)
		}
	}
	if _, err := PrepareUserUpdates(requests.UserUpdateRequest{}, "root"); err == nil {
		t.Fatal("invalid role accepted")
	}
	updates, err = PrepareUserUpdates(requests.UserUpdateRequest{Name: " Ana ", Email: " ANA@EXAMPLE.TEST ", Password: "StrongTest123!"}, "moderator")
	if err != nil {
		t.Fatal(err)
	}
	if updates["name"] != "Ana" || updates["email"] != "ana@example.test" || updates["role"] != "moderator" {
		t.Fatalf("incorrect patch: %+v", updates)
	}
	if bcrypt.CompareHashAndPassword([]byte(updates["password"].(string)), []byte("StrongTest123!")) != nil {
		t.Fatal("password is not correctly hashed")
	}
	updates, err = PrepareUserUpdates(requests.UserUpdateRequest{Name: "Ana"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := updates["role"]; ok {
		t.Fatal("self-service patch contains role")
	}
}

func TestCoordinatesAndTokens(t *testing.T) {
	for _, c := range []struct {
		lat, lng float64
		valid    bool
	}{{0, 0, true}, {90, 180, true}, {-90, -180, true}, {90.00001, 0, false}, {0, -180.00001, false}, {math.NaN(), 0, false}, {0, math.Inf(1), false}} {
		if ValidCoordinates(c.lat, c.lng) != c.valid {
			t.Fatalf("incorrect coordinate decision: %+v", c)
		}
	}
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		token, err := generateToken()
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := hex.DecodeString(token)
		if err != nil || len(decoded) != 32 || seen[token] {
			t.Fatal("invalid or duplicate token")
		}
		seen[token] = true
	}
}
