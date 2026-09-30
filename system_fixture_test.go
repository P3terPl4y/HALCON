package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	socket "github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/middleware/csrf"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/gofiber/template/html/v3"
	frameworkmigration "github.com/goravel/framework/database/migration"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"goravel/app/facades"
	ws "goravel/app/websockets"
	"goravel/bootstrap"
	"goravel/routes"
)

type systemMetrics struct {
	mu        sync.Mutex
	Requests  int         `json:"http_requests"`
	Statuses  map[int]int `json:"http_statuses"`
	durations []time.Duration
}

func (m *systemMetrics) record(status int, elapsed time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Requests++
	m.Statuses[status]++
	m.durations = append(m.durations, elapsed)
}
func (m *systemMetrics) percentiles() map[string]float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := append([]time.Duration(nil), m.durations...)
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	out := map[string]float64{}
	for name, p := range map[string]float64{"p50": .5, "p95": .95, "p99": .99} {
		if len(v) > 0 {
			out[name] = float64(v[int(float64(len(v)-1)*p)]) / float64(time.Millisecond)
		}
	}
	return out
}

type systemFixture struct {
	base      string
	app       *fiber.App
	hub       *ws.Hub
	metrics   *systemMetrics
	transport *http.Transport
}
type simulatedActor struct {
	f                     *systemFixture
	client                *http.Client
	ip                    string
	ID, PersonalID        uint
	Email, Password, Role string
}
type observedResponse struct {
	status   int
	body     []byte
	location string
	header   http.Header
}

// Each opt-in database suite is run in a separate go test process. No production
// tables are touched, and current_schema is checked before running migrations.
func newSystemFixture(t *testing.T) *systemFixture {
	t.Helper()
	pg, err := pgx.ParseConfig("sslmode=disable")
	require.NoError(t, err)
	cfg := facades.Config()
	pg.Host = cfg.GetString("database.connections.postgres.host")
	pg.Port = uint16(cfg.GetInt("database.connections.postgres.port"))
	pg.Database = cfg.GetString("database.connections.postgres.database")
	pg.User = cfg.GetString("database.connections.postgres.username")
	pg.Password = cfg.GetString("database.connections.postgres.password")
	conn, err := pgx.ConnectConfig(context.Background(), pg)
	require.NoError(t, err)
	schema := fmt.Sprintf("halcon_system_%d", time.Now().UnixNano())
	_, err = conn.Exec(context.Background(), "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := conn.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, err)
		conn.Close(context.Background())
	})
	cfg.Add("database.connections.postgres.schema", schema)
	cfg.Add("database.pool.max_open_conns", 32)
	bootstrap.Boot()
	var current struct{ Name string }
	require.NoError(t, facades.Orm().Query().Raw("SELECT current_schema() AS name").Scan(&current))
	require.Equal(t, schema, current.Name)
	require.NoError(t, frameworkmigration.NewMigrator(facades.Artisan(), facades.Schema(), "migrations").Run())
	engine := html.New("./app/views", ".html")
	engine.AddFunc("add", func(a, b int) int { return a + b })
	engine.AddFunc("sub", func(a, b int) int { return a - b })
	engine.AddFunc("deref", func(p *uint) uint {
		if p == nil {
			return 0
		}
		return *p
	})
	app := fiber.New(fiber.Config{Views: engine, TrustProxy: true, ProxyHeader: fiber.HeaderXForwardedFor, TrustProxyConfig: fiber.TrustProxyConfig{Loopback: true}})
	app.Use(recover.New())
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
	transport := &http.Transport{MaxIdleConns: 128, MaxIdleConnsPerHost: 64, MaxConnsPerHost: 64, IdleConnTimeout: time.Minute}
	f := &systemFixture{base: "http://" + listener.Addr().String(), app: app, hub: hub, metrics: &systemMetrics{Statuses: map[int]int{}}, transport: transport}
	t.Cleanup(func() {
		transport.CloseIdleConnections()
		require.NoError(t, app.Shutdown())
		require.NoError(t, <-ended)
	})
	oldLog := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(oldLog) })
	return f
}
func (f *systemFixture) actor(index int) *simulatedActor {
	jar, _ := cookiejar.New(nil)
	return &simulatedActor{f: f, ip: fmt.Sprintf("10.77.%d.%d", index/250, index%250+1), client: &http.Client{Jar: jar, Transport: f.transport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}
func (a *simulatedActor) request(method, path string, values url.Values) (observedResponse, error) {
	var body io.Reader
	if values != nil {
		body = strings.NewReader(values.Encode())
	}
	req, err := http.NewRequest(method, a.f.base+path, body)
	if err != nil {
		return observedResponse{}, err
	}
	req.Header.Set("X-Forwarded-For", a.ip)
	if values != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", a.f.base)
	}
	start := time.Now()
	r, err := a.client.Do(req)
	if err != nil {
		a.f.metrics.record(0, time.Since(start))
		return observedResponse{}, err
	}
	defer r.Body.Close()
	data, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	a.f.metrics.record(r.StatusCode, time.Since(start))
	return observedResponse{r.StatusCode, data, r.Header.Get("Location"), r.Header}, err
}
func (a *simulatedActor) sessionToken() (string, error) {
	r, err := a.request("GET", "/api/session", nil)
	if err != nil {
		return "", err
	}
	var s struct {
		Token string `json:"csrf_token"`
	}
	if r.status != 200 {
		return "", fmt.Errorf("session returned %d", r.status)
	}
	if err = json.Unmarshal(r.body, &s); err != nil {
		return "", err
	}
	if s.Token == "" {
		return "", fmt.Errorf("missing CSRF token")
	}
	return s.Token, nil
}
func (a *simulatedActor) post(path string, values url.Values) (observedResponse, error) {
	token, err := a.sessionToken()
	if err != nil {
		return observedResponse{}, err
	}
	copy := url.Values{}
	for k, v := range values {
		copy[k] = append([]string(nil), v...)
	}
	copy.Set("_csrf", token)
	return a.request("POST", path, copy)
}
func (a *simulatedActor) login() error {
	r, err := a.post("/api/auth/login", url.Values{"email": {a.Email}, "password": {a.Password}})
	if err != nil {
		return err
	}
	if r.status != 200 {
		return fmt.Errorf("login returned %d: %.200s", r.status, r.body)
	}
	var s struct {
		User struct {
			ID   uint   `json:"id"`
			Role string `json:"role"`
		} `json:"user"`
	}
	if err = json.Unmarshal(r.body, &s); err != nil {
		return err
	}
	a.ID = s.User.ID
	a.Role = s.User.Role
	return nil
}
func (a *simulatedActor) dial(path, origin string) (*socket.Conn, *http.Response, error) {
	base, _ := url.Parse(a.f.base)
	header := http.Header{"Origin": {origin}}
	for _, cookie := range a.client.Jar.Cookies(base) {
		header.Add("Cookie", cookie.String())
	}
	d := socket.Dialer{HandshakeTimeout: 10 * time.Second}
	return d.Dial("ws://"+base.Host+path, header)
}
func (a *simulatedActor) dashboard() (*socket.Conn, error) {
	c, r, err := a.dial(fmt.Sprintf("/dashboard/%d/", a.ID), a.f.base)
	if err != nil {
		if r != nil {
			r.Body.Close()
		}
		return nil, err
	}
	return c, nil
}
func expectResponse(t *testing.T, r observedResponse, err error, status int) {
	t.Helper()
	require.NoError(t, err)
	require.Equal(t, status, r.status, "response %.300s", r.body)
}
func expectRedirect(t *testing.T, r observedResponse, err error) {
	t.Helper()
	require.NoError(t, err)
	require.Contains(t, []int{302, 303}, r.status, "response %.300s", r.body)
}
