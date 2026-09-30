package controllers

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"goravel/app/models"
)

func TestIDsAndPagination(t *testing.T) {
	for _, c := range []struct {
		input string
		id    uint
	}{{"1", 1}, {"0", 0}, {"-1", 0}, {"1.2", 0}, {"' OR 1=1", 0}, {"18446744073709551616", 0}, {"", 0}} {
		if got := parseIDParam(c.input); got != c.id {
			t.Errorf("%q => %d", c.input, got)
		}
	}
	for _, c := range []struct {
		input string
		n     int
	}{{"", 20}, {"bad", 20}, {"3", 3}, {"999999999999999999999999", 20}} {
		if got := atoiDefault(c.input, 20); got != c.n {
			t.Errorf("%q => %d", c.input, got)
		}
	}
}

func TestPublicUserDoesNotExposeSecrets(t *testing.T) {
	if publicUser(nil) != nil {
		t.Fatal("nil user should be null")
	}
	u := models.User{Name: "Ana", Email: "ana@example.test", Password: "SECRET", Role: "user"}
	u.ID = 7
	data, err := json.Marshal(publicUser(&u))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "SECRET") || strings.Contains(string(data), "Password") {
		t.Fatal("password leaked")
	}
	if !strings.Contains(string(data), `"id":7`) {
		t.Fatal("missing id")
	}
}

func TestRecipientIDParsing(t *testing.T) {
	for _, c := range []struct {
		raw   string
		valid bool
	}{{"0", true}, {"1", true}, {"-1", false}, {"1.5", false}, {"18446744073709551616", false}, {"not-a-number", false}} {
		app := fiber.New()
		app.Post("/", func(ctx fiber.Ctx) error {
			_, err := recipientFromRequest(ctx)
			if (err == nil) != c.valid {
				t.Errorf("%q error=%v", c.raw, err)
			}
			return ctx.SendStatus(204)
		})
		req := httptest.NewRequest("POST", "/", strings.NewReader("recipient_id="+c.raw))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
	}
}
