package routes

import (
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

// RegisterReact publishes the built client on the same origin as its API.
// Source-only installations continue to serve the existing Goravel views.
func RegisterReact(app *fiber.App, directory string) {
	if info, err := os.Stat(filepath.Join(directory, "index.html")); err != nil || info.IsDir() {
		return
	}
	app.Get("/app", func(c fiber.Ctx) error {
		if c.Path() == "/app/" {
			return c.Next()
		}
		return c.Redirect().To("/app/")
	})
	app.Use("/app", static.New(directory))
}
