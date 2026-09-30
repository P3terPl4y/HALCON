package routes

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func TestReactDeploymentRoutes(t *testing.T) {
	directory := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(directory, "assets"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "index.html"), []byte("<main>HALCON React</main>"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "assets", "client.js"), []byte("console.log('HALCON')"), 0600))
	app := fiber.New()
	RegisterReact(app, directory)
	app.Get("/api/session", func(c fiber.Ctx) error { return c.JSON(fiber.Map{"user": nil}) })
	for _, item := range []struct {
		path     string
		status   int
		contains string
	}{
		{"/app", 303, ""}, {"/app/", 200, "HALCON React"}, {"/app/assets/client.js", 200, "HALCON"},
		{"/app/assets/missing.js", 404, ""}, {"/api/session", 200, "user"},
	} {
		r, err := app.Test(httptest.NewRequest("GET", item.path, nil))
		require.NoError(t, err)
		body, err := io.ReadAll(r.Body)
		r.Body.Close()
		require.NoError(t, err)
		require.Equal(t, item.status, r.StatusCode, item.path)
		require.Contains(t, string(body), item.contains)
		if item.path == "/app" {
			require.Equal(t, "/app/", r.Header.Get("Location"))
		}
	}
}

func TestReactAbsentBuildPreservesExistingRoutes(t *testing.T) {
	app := fiber.New()
	RegisterReact(app, t.TempDir())
	r, err := app.Test(httptest.NewRequest("GET", "/app/", nil))
	require.NoError(t, err)
	defer r.Body.Close()
	require.Equal(t, 404, r.StatusCode)
}
