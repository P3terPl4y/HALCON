package websocket

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/contrib/v3/websocket"
)

func TestPublishRejectsStaleProducerAndPersistenceFailure(t *testing.T) {
	h := NewHub()
	hc := &HalconConnection{HalconID: 1}
	calls := 0
	if err := h.Publish(hc, Coordinate{}, func() error { calls++; return nil }); !errors.Is(err, websocket.ErrCloseSent) || calls != 0 {
		t.Fatal("unregistered producer persisted")
	}
	h.RegisterHalcon(hc)
	dc := &DashboardConnection{Updates: make(chan Coordinate, 1)}
	h.RegisterDashboard(dc)
	want := errors.New("database unavailable")
	if err := h.Publish(hc, Coordinate{}, func() error { return want }); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if len(dc.Updates) != 0 {
		t.Fatal("failed persistence broadcast")
	}
	if err := h.Publish(hc, Coordinate{HalconID: 1}, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	coord := <-dc.Updates
	if coord.HalconID != 1 || coord.Timestamp <= 0 {
		t.Fatal("missing coordinate identity or time")
	}
	h.UnregisterDashboard(dc)
	if err := h.Publish(hc, Coordinate{}, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(dc.Updates) != 0 {
		t.Fatal("unregistered dashboard received data")
	}
}

func TestFullDashboardQueueDoesNotBlockOtherViewers(t *testing.T) {
	h := NewHub()
	hc := &HalconConnection{HalconID: 1}
	h.RegisterHalcon(hc)
	slow := &DashboardConnection{Updates: make(chan Coordinate, 1)}
	fast := &DashboardConnection{Updates: make(chan Coordinate, 2)}
	h.RegisterDashboard(slow)
	h.RegisterDashboard(fast)
	slow.Updates <- Coordinate{}
	done := make(chan error, 1)
	go func() { done <- h.Publish(hc, Coordinate{HalconID: 7}, func() error { return nil }) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("slow client blocked broadcast")
	}
	if (<-fast.Updates).HalconID != 7 {
		t.Fatal("healthy client missed data")
	}
}

func TestConcurrentDashboardLifecycle(t *testing.T) {
	h := NewHub()
	hc := &HalconConnection{HalconID: 1}
	h.RegisterHalcon(hc)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dc := &DashboardConnection{Updates: make(chan Coordinate, 1)}
			h.RegisterDashboard(dc)
			if err := h.Publish(hc, Coordinate{}, func() error { return nil }); err != nil {
				t.Error(err)
			}
			h.UnregisterDashboard(dc)
		}()
	}
	wg.Wait()
	if len(h.dashboards) != 0 {
		t.Fatal("dashboard registration leaked")
	}
}

func TestBrowserOriginPolicy(t *testing.T) {
	for _, c := range []struct {
		raw   string
		valid bool
	}{{"https://halcon.example", true}, {"", false}, {"null", false}, {"http://halcon.example", false}, {"https://attacker.example", false}, {"https://halcon.example:444", false}, {"https://user@halcon.example", false}, {"https://halcon.example/path", false}, {"https://halcon.example?token=x", false}, {"https://halcon.example#fragment", false}, {"%broken", false}} {
		if got := validBrowserOrigin(c.raw, "halcon.example", "https"); got != c.valid {
			t.Errorf("origin %q: %v", c.raw, got)
		}
	}
}

func TestSessionValidityFailsClosed(t *testing.T) {
	conn := &websocket.Conn{}
	if sessionValid(conn) {
		t.Fatal("missing session accepted")
	}
	conn.Locals("session_expires", time.Now().Add(time.Minute))
	conn.Locals("session_valid", func() bool { return true })
	if !sessionValid(conn) {
		t.Fatal("live session rejected")
	}
	conn.Locals("session_expires", time.Now().Add(-time.Second))
	if sessionValid(conn) {
		t.Fatal("expired session accepted")
	}
	conn.Locals("session_expires", time.Now().Add(time.Minute))
	conn.Locals("session_valid", func() bool { return false })
	if sessionValid(conn) {
		t.Fatal("revoked session accepted")
	}
}

func FuzzDecodeLocation(f *testing.F) {
	for _, seed := range []string{`{"latitude":0,"longitude":0}`, `{"latitude":90,"longitude":180}`, `null`, `[]`, `{"latitude":1e400}`, `{"latitude":null,"longitude":0}`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data string) {
		coord, ok := decodeLocation([]byte(data))
		if ok && (coord.Latitude < -90 || coord.Latitude > 90 || coord.Longitude < -180 || coord.Longitude > 180) {
			t.Fatal("out-of-range coordinate accepted")
		}
	})
}
