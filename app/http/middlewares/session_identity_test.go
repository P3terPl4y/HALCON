package middlewares

import (
	"math"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestSessionUserID(t *testing.T) {
	for _, c := range []struct {
		value any
		id    uint
		ok    bool
	}{{uint(1), 1, true}, {1, 1, true}, {int64(7), 7, true}, {float64(42), 42, true}, {nil, 0, false}, {"1", 0, false}, {true, 0, false}, {uint(0), 0, false}, {-1, 0, false}, {int64(-3), 0, false}, {1.5, 0, false}, {math.NaN(), 0, false}, {math.Inf(1), 0, false}, {9007199254740992.0, 0, false}} {
		id, ok := SessionUserID(c.value)
		if id != c.id || ok != c.ok {
			t.Errorf("%#v => %d %v; expected %d %v", c.value, id, ok, c.id, c.ok)
		}
	}
}

func TestRoleMiddlewareRejectsMissingIdentity(t *testing.T) {
	for _, handler := range []fiber.Handler{AdminMiddleware(), ModeratorMiddleware()} {
		app := fiber.New()
		app.Get("/", handler, func(c fiber.Ctx) error { return c.SendStatus(200) })
		r, err := app.Test(httptest.NewRequest("GET", "/", nil))
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != 401 {
			t.Fatalf("missing identity: %d", r.StatusCode)
		}
	}
}
