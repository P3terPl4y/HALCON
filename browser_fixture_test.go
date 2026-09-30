package main

import (
	"context"
	"fmt"
	frameworkmigration "github.com/goravel/framework/database/migration"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"goravel/app/facades"
	"goravel/app/models"
	"goravel/app/requests"
	"goravel/app/services"
	"goravel/bootstrap"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// Browser tests use the real server and a disposable schema. Never seed production.
func TestBrowserFixture(t *testing.T) {
	if os.Getenv("HALCON_BROWSER_FIXTURE") != "1" {
		t.Skip("opt-in fixture for browser checks")
	}
	cfg := facades.Config()
	pg, err := pgx.ParseConfig("sslmode=disable")
	require.NoError(t, err)
	pg.Host = cfg.GetString("database.connections.postgres.host")
	pg.Port = uint16(cfg.GetInt("database.connections.postgres.port"))
	pg.Database = cfg.GetString("database.connections.postgres.database")
	pg.User = cfg.GetString("database.connections.postgres.username")
	pg.Password = cfg.GetString("database.connections.postgres.password")
	conn, err := pgx.ConnectConfig(context.Background(), pg)
	require.NoError(t, err)
	defer conn.Close(context.Background())
	schema := fmt.Sprintf("halcon_browser_%d", time.Now().UnixNano())
	_, err = conn.Exec(context.Background(), "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	defer func() {
		_, err := conn.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, err)
	}()
	cfg.Add("database.connections.postgres.schema", schema)
	bootstrap.Boot()
	var current struct{ Name string }
	require.NoError(t, facades.Orm().Query().Raw("SELECT current_schema() AS name").Scan(&current))
	require.Equal(t, schema, current.Name, "refuse to touch a non-test schema")
	require.NoError(t, frameworkmigration.NewMigrator(facades.Artisan(), facades.Schema(), "migrations").Run())
	password, err := bcrypt.GenerateFromPassword([]byte("BrowserTest123!"), bcrypt.DefaultCost)
	require.NoError(t, err)
	for i, role := range []string{"user", "user", "moderator"} {
		name := []string{"Alicia de prueba", "Bruno <img src=x onerror=window.__xss=1>", "Moderador de prueba"}[i]
		u := models.User{Name: name, Email: fmt.Sprintf("browser%d@example.test", i), Phone: fmt.Sprintf("browser%d", i), Password: string(password), Status: true, Role: role}
		require.NoError(t, facades.Orm().Query().Create(&u))
		_, err := services.NewHalconService().EnsurePersonal(u.ID)
		require.NoError(t, err)
		if role == "moderator" {
			_, err = services.NewHalconService().Create(&requests.CreateHalconRequest{Name: "Dispositivo del moderador", ModeratorID: u.ID})
			require.NoError(t, err)
		}
	}
	for i := 4; i <= 120; i++ {
		email := fmt.Sprintf("browser-extra-%d@example.test", i)
		if i == 120 {
			email = "browser-last@example.test"
		}
		u := models.User{Name: "Usuario adicional de prueba", Email: email, Phone: fmt.Sprintf("browser-extra-%d", i), Password: string(password), Status: true, Role: "user"}
		require.NoError(t, facades.Orm().Query().Create(&u))
	}
	binary := os.Getenv("HALCON_BROWSER_BINARY")
	require.NotEmpty(t, binary)
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(), "DB_SCHEMA="+schema, "APP_PORT=3301", "APP_ENV=local")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	require.NoError(t, cmd.Start())
	defer func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	}()
	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(os.Getenv("HALCON_BROWSER_STOP_FILE")); err == nil {
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatal("browser fixture timed out")
}
