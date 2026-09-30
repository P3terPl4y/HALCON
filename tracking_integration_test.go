package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"goravel/app/facades"
	"goravel/app/models"
	"goravel/app/requests"
	"goravel/app/services"
	ws "goravel/app/websockets"
	"goravel/bootstrap"
	"goravel/routes"

	socket "github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/middleware/csrf"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/template/html/v3"
	frameworkmigration "github.com/goravel/framework/database/migration"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// Opt-in: uses a temporary PostgreSQL schema, never the application's tables.
func TestTrackingIntegration(t *testing.T) {
	if os.Getenv("HALCON_INTEGRATION_TEST") != "1" {
		t.Skip("set HALCON_INTEGRATION_TEST=1 for isolated PostgreSQL integration")
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
	schema := fmt.Sprintf("halcon_test_%d", time.Now().UnixNano())
	_, err = conn.Exec(context.Background(), "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	defer func() {
		_, e := conn.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, e)
	}()
	cfg.Add("database.connections.postgres.schema", schema)
	bootstrap.Boot()
	var current struct{ Name string }
	require.NoError(t, facades.Orm().Query().Raw("SELECT current_schema() AS name").Scan(&current))
	require.Equal(t, schema, current.Name, "refuse to touch a non-test schema")
	migrator := frameworkmigration.NewMigrator(facades.Artisan(), facades.Schema(), "migrations")
	require.NoError(t, migrator.Run())
	require.NoError(t, migrator.Run(), "running the migrator again must be safe")
	statuses, err := migrator.Status()
	require.NoError(t, err)
	require.Len(t, statuses, len(bootstrap.Migrations()))
	for _, status := range statuses {
		require.True(t, status.Ran, status.Name)
	}
	hs := services.NewHalconService()
	password, err := bcrypt.GenerateFromPassword([]byte("TestPass123!"), bcrypt.MinCost)
	require.NoError(t, err)
	users := make([]models.User, 5)
	for i, role := range []string{"user", "user", "user", "moderator", "moderator"} {
		users[i] = models.User{Name: fmt.Sprintf("Tracking %d", i), Email: fmt.Sprintf("tracking%d@example.test", i), Phone: fmt.Sprintf("test%d", i), Password: string(password), Status: true, Role: role}
		require.NoError(t, facades.Orm().Query().Create(&users[i]))
	}
	var wg sync.WaitGroup
	ids := make(chan uint, 8)
	failures := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h, e := hs.EnsurePersonal(users[0].ID)
			if e != nil {
				failures <- e
			} else {
				ids <- h.ID
			}
		}()
	}
	wg.Wait()
	close(ids)
	close(failures)
	for e := range failures {
		require.NoError(t, e)
	}
	personal, err := hs.EnsurePersonal(users[0].ID)
	require.NoError(t, err)
	for id := range ids {
		require.Equal(t, personal.ID, id)
	}
	require.Error(t, hs.SetRecipient(users[0].ID, users[0].ID))
	require.Error(t, hs.SetRecipient(users[0].ID, 999999))
	require.NoError(t, hs.SetRecipient(users[0].ID, users[1].ID))
	visible, err := hs.VisibleTo(users[1].ID)
	require.NoError(t, err)
	require.Len(t, visible, 1)
	require.NoError(t, hs.SetRecipient(users[0].ID, users[2].ID))
	visible, err = hs.VisibleTo(users[1].ID)
	require.NoError(t, err)
	require.Empty(t, visible)
	device, err := hs.Create(&requests.CreateHalconRequest{Name: "Device", ModeratorID: users[3].ID})
	require.NoError(t, err)
	visible, err = hs.VisibleTo(users[3].ID)
	require.NoError(t, err)
	require.Len(t, visible, 1, "unassigned devices must be visible to their moderator")
	_, err = hs.Assign(device.ID, users[1].ID, users[4].ID, "")
	require.Error(t, err)
	_, err = hs.Assign(device.ID, users[1].ID, users[3].ID, "")
	require.NoError(t, err)
	_, err = hs.Assign(personal.ID, users[1].ID, users[3].ID, "")
	require.Error(t, err)
	stats, err := hs.StatsByModerator(users[3].ID, false)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats["assigned"])
	stats, err = hs.StatsByModerator(users[4].ID, false)
	require.NoError(t, err)
	require.Equal(t, int64(0), stats["assigned"])
	require.NoError(t, hs.UpdateLastLocation(device.ID, 12.1234567, -123.7654321))
	saved, err := hs.GetByID(device.ID)
	require.NoError(t, err)
	require.InDelta(t, 12.1234567, saved.LastLat, 0.00000001)
	require.InDelta(t, -123.7654321, saved.LastLng, 0.00000001)

	engine := html.New("./app/views", ".html")
	engine.AddFunc("add", func(a, b int) int { return a + b })
	engine.AddFunc("sub", func(a, b int) int { return a - b })
	engine.AddFunc("deref", func(v *uint) uint {
		if v == nil {
			return 0
		}
		return *v
	})
	app := fiber.New(fiber.Config{Views: engine})
	middleware, store := session.NewWithStore()
	app.Use(middleware)
	app.Use(csrf.New(csrf.Config{Session: store, Extractor: extractors.FromForm("_csrf")}))
	hub := ws.NewHub()
	ws.RegisterRoutes(app, hub, store)
	routes.Web(app)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ended := make(chan error, 1)
	go func() { ended <- app.Listener(listener) }()
	defer func() { require.NoError(t, app.Shutdown()); require.NoError(t, <-ended) }()
	base := "http://" + listener.Addr().String()
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	post := func(client *http.Client, path string, values url.Values) (*http.Response, error) {
		formPath := "/profile"
		if path == "/login" {
			formPath = "/login"
		}
		form, e := client.Get(base + formPath)
		require.NoError(t, e)
		body, e := io.ReadAll(form.Body)
		form.Body.Close()
		require.NoError(t, e)
		match := regexp.MustCompile(`name="_csrf" value="([^"]+)"`).FindSubmatch(body)
		require.Len(t, match, 2, "form must render its CSRF token")
		values.Set("_csrf", string(match[1]))
		return client.PostForm(base+path, values)
	}
	response, err := post(client, "/login", url.Values{"email": {users[0].Email}, "password": {"TestPass123!"}})
	require.NoError(t, err)
	require.Equal(t, 200, response.StatusCode)
	response.Body.Close()
	dialFor := func(client *http.Client, path string) (*socket.Conn, *http.Response, error) {
		parsed, _ := url.Parse(base)
		header := http.Header{"Origin": {base}}
		for _, cookie := range client.Jar.Cookies(parsed) {
			header.Add("Cookie", cookie.String())
		}
		return socket.DefaultDialer.Dial("ws://"+listener.Addr().String()+path, header)
	}
	_, rejected, dialErr := socket.DefaultDialer.Dial("ws://"+listener.Addr().String()+"/location", http.Header{"Origin": {base}})
	require.Error(t, dialErr)
	require.Equal(t, http.StatusUnauthorized, rejected.StatusCode)
	rejected.Body.Close()
	parsedBase, _ := url.Parse(base)
	crossHeaders := http.Header{"Origin": {"https://unrelated.example"}}
	for _, cookie := range jar.Cookies(parsedBase) {
		crossHeaders.Add("Cookie", cookie.String())
	}
	_, rejected, dialErr = socket.DefaultDialer.Dial("ws://"+listener.Addr().String()+"/location", crossHeaders)
	require.Error(t, dialErr)
	require.Equal(t, http.StatusForbidden, rejected.StatusCode)
	rejected.Body.Close()
	loginAs := func(user models.User) *http.Client {
		jar, e := cookiejar.New(nil)
		require.NoError(t, e)
		client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
		response, e := post(client, "/login", url.Values{"email": {user.Email}, "password": {"TestPass123!"}})
		require.NoError(t, e)
		require.Equal(t, 200, response.StatusCode)
		response.Body.Close()
		return client
	}
	// Security assertions exercise the same handlers as the React client.
	response, err = client.Get(base + "/users")
	require.NoError(t, err)
	require.Equal(t, 403, response.StatusCode, "ordinary users cannot enumerate accounts")
	response.Body.Close()
	response, err = (&http.Client{Timeout: 10 * time.Second}).Get(base + "/api/tracking")
	require.NoError(t, err)
	require.Equal(t, 401, response.StatusCode)
	response.Body.Close()
	response, err = client.Get(base + "/api/tracking")
	require.NoError(t, err)
	require.Equal(t, 200, response.StatusCode)
	require.Equal(t, "no-store", response.Header.Get("Cache-Control"))
	apiBody, err := io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.NotContains(t, string(apiBody), `"Password"`)
	require.NotContains(t, string(apiBody), `"Token"`)
	for _, values := range []url.Values{
		{"recipient_id": {fmt.Sprint(users[0].ID)}},
		{"recipient_id": {"-1"}},
		{"recipient_email": {"' OR 1=1 --"}},
		{"recipient_email": {"nonexistent@example.test"}},
	} {
		response, err = post(client, "/api/tracking/recipient", values)
		require.NoError(t, err)
		require.Equal(t, 422, response.StatusCode)
		response.Body.Close()
	}
	require.NoError(t, services.NewUserService().Update(users[1].ID, map[string]any{"status": false}))
	response, err = post(client, "/api/tracking/recipient", url.Values{"recipient_email": {users[1].Email}})
	require.NoError(t, err)
	require.Equal(t, 422, response.StatusCode)
	response.Body.Close()
	require.NoError(t, services.NewUserService().Update(users[1].ID, map[string]any{"status": true}))
	response, err = post(client, "/api/tracking/recipient", url.Values{"recipient_email": {strings.ToUpper(users[2].Email)}, "owner_id": {fmt.Sprint(users[1].ID)}})
	require.NoError(t, err)
	require.Equal(t, 200, response.StatusCode)
	response.Body.Close()
	unchanged, err := hs.EnsurePersonal(users[0].ID)
	require.NoError(t, err)
	require.Equal(t, users[2].ID, *unchanged.RecipientID)
	wrong, _, err := dialFor(client, fmt.Sprintf("/dashboard/%d/", users[1].ID))
	require.NoError(t, err)
	wrong.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err = wrong.ReadMessage()
	require.Error(t, err)
	wrong.Close()
	outsideClient := loginAs(users[4])
	response, err = post(outsideClient, "/moderator/halcones/assign", url.Values{"halcon_id": {fmt.Sprint(device.ID)}, "user_id": {fmt.Sprint(users[2].ID)}, "moderator_id": {fmt.Sprint(users[3].ID)}})
	require.NoError(t, err)
	response.Body.Close()
	assignment, err := hs.GetActiveAssignment(device.ID)
	require.NoError(t, err)
	require.Equal(t, users[1].ID, assignment.UserID, "forged actor cannot reassign another moderator's device")
	response, err = outsideClient.Get(base + "/api/session")
	require.NoError(t, err)
	var apiSession struct {
		Token string `json:"csrf_token"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&apiSession))
	response.Body.Close()
	del, err := http.NewRequest(http.MethodDelete, base+fmt.Sprintf("/moderator/halcones/%d", device.ID), strings.NewReader(url.Values{"_csrf": {apiSession.Token}}.Encode()))
	require.NoError(t, err)
	del.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err = outsideClient.Do(del)
	require.NoError(t, err)
	require.Equal(t, 403, response.StatusCode)
	response.Body.Close()
	_, err = hs.GetByID(device.ID)
	require.NoError(t, err)
	response, err = post(outsideClient, fmt.Sprintf("/moderator/halcones/%d/delete", device.ID), url.Values{})
	require.NoError(t, err)
	require.Equal(t, 403, response.StatusCode, "native form deletion also checks ownership")
	response.Body.Close()
	_, err = hs.GetByID(device.ID)
	require.NoError(t, err)
	newJar, err := cookiejar.New(nil)
	require.NoError(t, err)
	publicClient := &http.Client{Jar: newJar, Timeout: 10 * time.Second}
	response, err = post(publicClient, "/register", url.Values{"name": {"Public test"}, "email": {"public@example.test"}, "phone": {"public-phone"}, "password": {"TestPass123!"}, "role": {"admin"}})
	require.NoError(t, err)
	require.Equal(t, 200, response.StatusCode)
	response.Body.Close()
	var registered models.User
	require.NoError(t, facades.Orm().Query().Where("email = ?", "public@example.test").FirstOrFail(&registered))
	require.Equal(t, "user", registered.Role)
	readLive := func(conn *socket.Conn, id uint) {
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(8*time.Second)))
		for i := 0; i < 12; i++ {
			var event map[string]any
			require.NoError(t, conn.ReadJSON(&event))
			if event["halcon_id"] == float64(id) && event["latitude"] == float64(0) {
				return
			}
		}
		t.Fatal("live coordinate was not delivered")
	}
	receiverClient := loginAs(users[2])
	receiver, _, err := dialFor(receiverClient, fmt.Sprintf("/dashboard/%d/", users[2].ID))
	require.NoError(t, err)
	defer receiver.Close()
	moderatorClient := loginAs(users[3])
	moderatorSocket, _, err := dialFor(moderatorClient, fmt.Sprintf("/dashboard/%d/", users[3].ID))
	require.NoError(t, err)
	defer moderatorSocket.Close()
	dashboard, _, err := dialFor(client, fmt.Sprintf("/dashboard/%d/", users[0].ID))
	require.NoError(t, err)
	defer dashboard.Close()
	producer, _, err := dialFor(client, "/location")
	require.NoError(t, err)
	defer producer.Close()
	require.NoError(t, producer.WriteJSON(map[string]float64{"latitude": 0, "longitude": 0}))
	require.NoError(t, dashboard.SetReadDeadline(time.Now().Add(8*time.Second)))
	got := false
	for i := 0; i < 10; i++ {
		var event map[string]any
		require.NoError(t, dashboard.ReadJSON(&event))
		if event["halcon_id"] == float64(personal.ID) && event["latitude"] == float64(0) {
			got = true
			break
		}
	}
	require.True(t, got, "live GPS update reaches owner")
	readLive(receiver, personal.ID)
	physical, _, err := socket.DefaultDialer.Dial(fmt.Sprintf("ws://%s/halcon/%d/%d/?token=%s", listener.Addr().String(), users[1].ID, device.ID, device.Token), nil)
	require.NoError(t, err)
	defer physical.Close()
	require.NoError(t, physical.WriteJSON(map[string]float64{"latitude": 0, "longitude": 0}))
	readLive(moderatorSocket, device.ID)
	response, err = client.PostForm(base+"/profile/share", url.Values{"recipient_id": {fmt.Sprint(users[1].ID)}})
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, response.StatusCode, "sharing requires a CSRF token")
	response.Body.Close()
	response, err = post(client, "/profile/share", url.Values{"recipient_id": {fmt.Sprint(users[1].ID)}})
	require.NoError(t, err)
	require.Equal(t, 200, response.StatusCode)
	response.Body.Close()
	receiver.SetReadDeadline(time.Now().Add(8 * time.Second))
	removed := false
	for i := 0; i < 12; i++ {
		var event struct {
			Type     string `json:"type"`
			Halcones []struct {
				ID uint `json:"halcon_id"`
			} `json:"halcones"`
		}
		require.NoError(t, receiver.ReadJSON(&event))
		if event.Type == "snapshot" {
			contains := false
			for _, h := range event.Halcones {
				if h.ID == personal.ID {
					contains = true
				}
			}
			if !contains {
				removed = true
				break
			}
		}
	}
	require.True(t, removed, "old recipient loses access without reconnecting")
	successor, _, err := dialFor(client, "/location")
	require.NoError(t, err)
	defer successor.Close()
	require.NoError(t, successor.WriteJSON(map[string]float64{"latitude": 0, "longitude": 0}))
	readLive(dashboard, personal.ID)
	require.Eventually(t, func() bool { h, e := hs.GetByID(personal.ID); return e == nil && h.IsActive }, 3*time.Second, 50*time.Millisecond)
	response, err = post(client, "/logout", url.Values{})
	require.NoError(t, err)
	response.Body.Close()
	require.NoError(t, successor.WriteJSON(map[string]float64{"latitude": 1, "longitude": 1}))
	successor.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err = successor.ReadMessage()
	require.Error(t, err, "logout revokes producer")
	if timeout, ok := err.(net.Error); ok {
		require.False(t, timeout.Timeout(), "must close, not merely time out")
	}
	limited := false
	for i := 0; i < 11; i++ {
		response, err = post(client, "/api/auth/login", url.Values{"email": {"unknown@example.test"}, "password": {"bad-password"}})
		require.NoError(t, err)
		status := response.StatusCode
		response.Body.Close()
		if status == 429 {
			limited = true
			break
		}
		require.Equal(t, 401, status)
	}
	require.True(t, limited, "credential attempts must be rate limited")

	// Rollback is exercised too; the test schema is discarded afterwards.
	require.NoError(t, migrator.Rollback(1, 0))
	// Recover the exact case where DDL committed but migration logging failed.
	require.NoError(t, bootstrap.Migrations()[4].Up())
	require.NoError(t, migrator.Run())
	statuses, err = migrator.Status()
	require.NoError(t, err)
	require.True(t, statuses[len(statuses)-1].Ran)
}
