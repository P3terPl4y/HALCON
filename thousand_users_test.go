package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	socket "github.com/fasthttp/websocket"
	"github.com/stretchr/testify/require"
	"goravel/app/facades"
	"goravel/app/models"
	"goravel/app/services"
)

type loadTracking struct {
	PersonalID uint `json:"personal_id"`
	Recipient  *struct {
		ID uint `json:"id"`
	} `json:"recipient"`
	Halcones []models.Halcon `json:"halcones"`
}
type loadState struct {
	ID          uint    `json:"halcon_id"`
	Name        string  `json:"name"`
	Lat         float64 `json:"lat"`
	Lng         float64 `json:"lng"`
	HasLocation bool    `json:"has_location"`
}
type loadMessage struct {
	Type     string      `json:"type"`
	ID       uint        `json:"halcon_id"`
	UserID   uint        `json:"user_id"`
	Lat      float64     `json:"latitude"`
	Lng      float64     `json:"longitude"`
	Halcones []loadState `json:"halcones"`
}

func readLoadMessage(c *socket.Conn, deadline time.Time) (loadMessage, error) {
	c.SetReadDeadline(deadline)
	var message loadMessage
	err := c.ReadJSON(&message)
	return message, err
}
func waitLoadCoordinate(c *socket.Conn, id, uid uint, lat float64) error {
	deadline := time.Now().Add(12 * time.Second)
	for {
		m, err := readLoadMessage(c, deadline)
		if err != nil {
			return err
		}
		if m.Type == "" && m.ID == id {
			if m.UserID != uid || m.Lat != lat {
				return fmt.Errorf("GPS identity or coordinates were forged")
			}
			return nil
		}
	}
}
func waitLoadRevocation(c *socket.Conn, id uint) error {
	deadline := time.Now().Add(12 * time.Second)
	for {
		m, err := readLoadMessage(c, deadline)
		if err != nil {
			return err
		}
		if m.Type == "snapshot" {
			found := false
			for _, h := range m.Halcones {
				if h.ID == id {
					found = true
				}
			}
			if !found {
				return nil
			}
		}
	}
}
func loadStatus(r observedResponse, err error, want int) error {
	if err != nil {
		return err
	}
	if r.status != want {
		return fmt.Errorf("HTTP %d, expected %d: %.160s", r.status, want, r.body)
	}
	return nil
}
func loadRedirect(r observedResponse, err error) error {
	if err != nil {
		return err
	}
	if r.status != 302 && r.status != 303 {
		return fmt.Errorf("expected redirect, got %d: %.160s", r.status, r.body)
	}
	return nil
}

func exerciseRoleWorkflow(i int, a *simulatedActor, actors []*simulatedActor, deviceIDs []uint) error {
	if a.Role == "user" {
		return nil
	}
	hs := services.NewHalconService()
	var device *models.Halcon
	var err error
	if a.Role == "moderator" {
		device, err = hs.GetByID(deviceIDs[i-2])
		if err != nil {
			return err
		}
		for _, path := range []string{"/moderator/halcones?limit=100", fmt.Sprintf("/moderator/halcones/%d/editor", device.ID)} {
			r, e := a.request("GET", path, nil)
			if e = loadStatus(r, e, 200); e != nil {
				return e
			}
		}
		foreign := deviceIDs[(i-1)%18]
		r, e := a.post("/moderator/halcones/assign", url.Values{"halcon_id": {fmt.Sprint(foreign)}, "user_id": {fmt.Sprint(actors[50].ID)}, "moderator_id": {fmt.Sprint(actors[(i+1)%20].ID)}})
		if e = loadRedirect(r, e); e != nil {
			return e
		}
		if !strings.Contains(r.location, "no_se_pudo_asignar") {
			return fmt.Errorf("moderator reassigned alien device")
		}
		r, e = a.post("/moderator/halcones/assign", url.Values{"halcon_id": {fmt.Sprint(device.ID)}, "user_id": {fmt.Sprint(actors[20+i-2].ID)}, "package_id": {"LOAD-REASSIGNED"}})
		if e = loadRedirect(r, e); e != nil {
			return e
		}
	} else {
		for _, path := range []string{"/admin/", "/admin/users?limit=100&page=2", "/admin/users/create", "/admin/halcones?limit=100", "/admin/halcones/create"} {
			r, e := a.request("GET", path, nil)
			if path == "/admin/" {
				e = loadRedirect(r, e)
			} else {
				e = loadStatus(r, e, 200)
			}
			if e != nil {
				return e
			}
		}
		email := fmt.Sprintf("temporary-admin-%d@example.test", i)
		r, e := a.post("/admin/users", url.Values{"name": {"Temporal de carga"}, "email": {email}, "phone": {fmt.Sprintf("temporary-admin-%d", i)}, "password": {"TemporaryLoad123!"}, "role": {"user"}})
		if e = loadRedirect(r, e); e != nil {
			return e
		}
		var user models.User
		if e = facades.Orm().Query().Where("email = ?", email).FirstOrFail(&user); e != nil {
			return e
		}
		r, e = a.request("GET", fmt.Sprintf("/admin/users/%d/edit", user.ID), nil)
		if e = loadStatus(r, e, 200); e != nil {
			return e
		}
		r, e = a.post(fmt.Sprintf("/admin/users/%d", user.ID), url.Values{"name": {"Temporal modificado"}, "role": {"moderator"}})
		if e = loadRedirect(r, e); e != nil {
			return e
		}
		r, e = a.post(fmt.Sprintf("/admin/users/%d/delete", user.ID), url.Values{})
		if e = loadRedirect(r, e); e != nil {
			return e
		}
		if _, e = services.NewUserService().GetByID(user.ID); e == nil {
			return fmt.Errorf("admin failed to delete temporary user")
		}
		name := fmt.Sprintf("Dispositivo admin carga %d", i)
		r, e = a.post("/admin/halcones", url.Values{"name": {name}})
		if e = loadRedirect(r, e); e != nil {
			return e
		}
		var h models.Halcon
		if e = facades.Orm().Query().Where("name = ?", name).FirstOrFail(&h); e != nil {
			return e
		}
		device = &h
		for _, suffix := range []string{"", "/edit", "/assign"} {
			r, e = a.request("GET", fmt.Sprintf("/admin/halcones/%d%s", h.ID, suffix), nil)
			if e = loadStatus(r, e, 200); e != nil {
				return e
			}
		}
		r, e = a.post(fmt.Sprintf("/admin/halcones/%d", h.ID), url.Values{"name": {name + " actualizado"}})
		if e = loadRedirect(r, e); e != nil {
			return e
		}
		r, e = a.post(fmt.Sprintf("/admin/halcones/%d/assign", h.ID), url.Values{"user_id": {fmt.Sprint(actors[40+i].ID)}, "package_id": {"LOAD-ADMIN"}})
		if e = loadRedirect(r, e); e != nil {
			return e
		}
	}
	assignment, err := hs.GetActiveAssignment(device.ID)
	if err != nil {
		return err
	}
	var receiver *simulatedActor
	for _, actor := range actors {
		if actor.ID == assignment.UserID {
			receiver = actor
			break
		}
	}
	if receiver == nil {
		return fmt.Errorf("unknown device recipient")
	}
	viewer, err := a.dashboard()
	if err != nil {
		return err
	}
	defer viewer.Close()
	if _, err = readLoadMessage(viewer, time.Now().Add(12*time.Second)); err != nil {
		return err
	}
	view, err := receiver.dashboard()
	if err != nil {
		return err
	}
	defer view.Close()
	if _, err = readLoadMessage(view, time.Now().Add(12*time.Second)); err != nil {
		return err
	}
	producer, r, err := a.dial(fmt.Sprintf("/halcon/%d/%d/?token=%s", receiver.ID, device.ID, device.Token), a.f.base)
	if err != nil {
		if r != nil {
			r.Body.Close()
		}
		return err
	}
	defer producer.Close()
	if err = producer.WriteJSON(map[string]any{"latitude": 20.25, "longitude": -70.75}); err != nil {
		return err
	}
	if err = waitLoadCoordinate(viewer, device.ID, receiver.ID, 20.25); err != nil {
		return err
	}
	if err = waitLoadCoordinate(view, device.ID, receiver.ID, 20.25); err != nil {
		return err
	}
	producer.Close()
	bad, r, err := a.dial(fmt.Sprintf("/halcon/%d/%d/?token=%s", a.ID, device.ID, device.Token), a.f.base)
	if err != nil {
		if r != nil {
			r.Body.Close()
		}
		return err
	}
	defer bad.Close()
	bad.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err = bad.ReadMessage()
	if _, ok := err.(*socket.CloseError); !ok {
		return fmt.Errorf("device accepted a forged assigned user")
	}
	if a.Role == "admin" {
		response, e := a.post(fmt.Sprintf("/admin/halcones/%d/delete", device.ID), url.Values{})
		return loadRedirect(response, e)
	}
	return nil
}

func TestThousandUsersIsolation(t *testing.T) {
	if os.Getenv("HALCON_LOAD_TEST") != "1" {
		t.Skip("opt-in 1000-user simulation; requires isolated PostgreSQL")
	}
	const users, workers = 1000, 32
	started := time.Now()
	f := newSystemFixture(t)
	us := services.NewUserService()
	hs := services.NewHalconService()
	actors := make([]*simulatedActor, users)
	for i := range actors {
		a := f.actor(i)
		a.Email = fmt.Sprintf("load%04d@example.test", i)
		a.Password = "LoadUsersTest123!"
		a.Role = "user"
		actors[i] = a
	}
	phases := map[string]float64{}
	failures := []string{}
	var rejected atomic.Int64
	defer func() {
		report := map[string]any{"users": users, "workers": workers, "roles": map[string]int{"admin": 2, "moderator": 18, "user": 980}, "elapsed_seconds": time.Since(started).Seconds(), "phases_seconds": phases, "http_requests": f.metrics.Requests, "http_statuses": f.metrics.Statuses, "http_latency_ms": f.metrics.percentiles(), "rejected_access_assertions": rejected.Load(), "failures": failures, "success": !t.Failed()}
		data, err := json.MarshalIndent(report, "", "  ")
		if err == nil {
			path := os.Getenv("HALCON_TEST_REPORT")
			if path == "" {
				path = "/tmp/halcon-load-1000-results.json"
			}
			require.NoError(t, os.WriteFile(path, data, 0600))
			t.Logf("report: %s; requests: %d; latency ms: %v", path, f.metrics.Requests, f.metrics.percentiles())
		}
	}()
	phase := func(name string, fn func(int, *simulatedActor) error) {
		t.Helper()
		begin := time.Now()
		jobs := make(chan int, users)
		errs := make(chan string, users)
		var wg sync.WaitGroup
		var completed atomic.Int64
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for index := range jobs {
					if err := fn(index, actors[index]); err != nil {
						errs <- fmt.Sprintf("%s actor %d: %v", name, index, err)
					}
					n := completed.Add(1)
					if n%100 == 0 {
						t.Logf("%s: %d/%d", name, n, users)
					}
				}
			}()
		}
		for i := 0; i < users; i++ {
			jobs <- i
		}
		close(jobs)
		wg.Wait()
		close(errs)
		for err := range errs {
			failures = append(failures, err)
		}
		phases[name] = time.Since(begin).Seconds()
		if len(failures) > 0 {
			for i, err := range failures {
				if i >= 20 {
					break
				}
				t.Error(err)
			}
			t.Fatalf("phase %s failed (%d failures)", name, len(failures))
		}
		t.Logf("%s finished in %.2fs", name, phases[name])
	}

	phase("registration_and_login", func(i int, a *simulatedActor) error {
		r, err := a.post("/register", url.Values{"name": {fmt.Sprintf("Usuario %04d", i)}, "email": {a.Email}, "phone": {fmt.Sprintf("load-phone-%04d", i)}, "password": {a.Password}, "role": {"admin"}})
		if err = loadRedirect(r, err); err != nil {
			return err
		}
		if err = a.login(); err != nil {
			return err
		}
		if a.Role != "user" {
			return fmt.Errorf("public registration escalated role")
		}
		r, err = a.request("GET", "/api/tracking", nil)
		if err = loadStatus(r, err, 200); err != nil {
			return err
		}
		var tracking loadTracking
		if err = json.Unmarshal(r.body, &tracking); err != nil {
			return err
		}
		a.PersonalID = tracking.PersonalID
		count, err := facades.Orm().Query().Model(&models.Halcon{}).Where("owner_id = ?", a.ID).Count()
		if err != nil || count != 1 {
			return fmt.Errorf("personal halcon uniqueness failed: %d %v", count, err)
		}
		return nil
	})
	// Only the trusted test operator grants roles. Public registration above
	// must have ignored the forged admin role for all 1000 accounts.
	for i := 0; i < 20; i++ {
		role := "moderator"
		if i < 2 {
			role = "admin"
		}
		require.NoError(t, us.AdminUpdateUser(actors[i].ID, map[string]any{"role": role}))
		actors[i].Role = role
	}
	deviceIDs := make([]uint, 18)
	for i := 2; i < 20; i++ {
		a := actors[i]
		r, err := a.post("/moderator/halcones", url.Values{"name": {fmt.Sprintf("Dispositivo carga %d", i)}, "moderator_id": {fmt.Sprint(actors[0].ID)}})
		require.NoError(t, loadRedirect(r, err))
		var h models.Halcon
		require.NoError(t, facades.Orm().Query().Where("name = ?", fmt.Sprintf("Dispositivo carga %d", i)).FirstOrFail(&h))
		require.Equal(t, a.ID, h.ModeratorID)
		deviceIDs[i-2] = h.ID
		r, err = a.post("/moderator/halcones/assign", url.Values{"halcon_id": {fmt.Sprint(h.ID)}, "user_id": {fmt.Sprint(actors[20+i-2].ID)}, "package_id": {"LOAD"}})
		require.NoError(t, loadRedirect(r, err))
	}

	phase("profile_sharing_and_access_attacks", func(i int, a *simulatedActor) error {
		other := actors[(i+1)%users]
		foreign := deviceIDs[i%18]
		if i >= 2 && i < 20 {
			foreign = deviceIDs[(i-1)%18]
		}
		for _, path := range []string{"/profile", "/profile/edit"} {
			r, err := a.request("GET", path, nil)
			if err = loadStatus(r, err, 200); err != nil {
				return err
			}
		}
		r, err := a.post("/profile/edit", url.Values{"name": {fmt.Sprintf("Actualizado %04d", i)}, "email": {strings.ToUpper(a.Email)}, "user_id": {fmt.Sprint(other.ID)}, "role": {"admin"}})
		if err = loadRedirect(r, err); err != nil {
			return err
		}
		r, err = a.post("/api/tracking/recipient", url.Values{"recipient_email": {other.Email}, "owner_id": {fmt.Sprint(other.ID)}})
		if err = loadStatus(r, err, 200); err != nil {
			return err
		}
		var tracking loadTracking
		if err = json.Unmarshal(r.body, &tracking); err != nil {
			return err
		}
		if tracking.Recipient == nil || tracking.Recipient.ID != other.ID || tracking.PersonalID != a.PersonalID {
			return fmt.Errorf("sharing identity mismatch")
		}
		for _, v := range []url.Values{{"recipient_id": {fmt.Sprint(a.ID)}}, {"recipient_email": {"' OR 1=1 --"}}, {"recipient_id": {"-1"}}} {
			r, err = a.post("/api/tracking/recipient", v)
			if err = loadStatus(r, err, 422); err != nil {
				return err
			}
			rejected.Add(1)
		}
		r, err = a.request("POST", "/api/tracking/recipient", url.Values{"recipient_id": {"0"}})
		if err = loadStatus(r, err, 403); err != nil {
			return err
		}
		rejected.Add(1)
		if a.Role != "admin" {
			for _, path := range []string{fmt.Sprintf("/admin/users/%d/edit", other.ID), fmt.Sprintf("/admin/halcones/%d", foreign)} {
				r, err = a.request("GET", path, nil)
				if err = loadStatus(r, err, 403); err != nil {
					return err
				}
				rejected.Add(1)
			}
			r, err = a.post(fmt.Sprintf("/admin/users/%d", other.ID), url.Values{"role": {"admin"}})
			if err = loadStatus(r, err, 403); err != nil {
				return err
			}
			rejected.Add(1)
			r, err = a.post(fmt.Sprintf("/moderator/halcones/%d/delete", foreign), url.Values{})
			if err = loadStatus(r, err, 403); err != nil {
				return err
			}
			rejected.Add(1)
		}
		if a.Role == "user" {
			r, err = a.request("GET", "/users", nil)
			if err = loadStatus(r, err, 403); err != nil {
				return err
			}
			rejected.Add(1)
		} else {
			r, err = a.request("GET", "/users?search=load0999%40example.test", nil)
			if err = loadStatus(r, err, 200); err != nil {
				return err
			}
			if !strings.Contains(string(r.body), actors[999].Email) {
				return fmt.Errorf("user outside first page cannot be selected")
			}
		}
		c, response, err := a.dial("/location", "https://attacker.example")
		if c != nil {
			c.Close()
		}
		if err == nil || response == nil || response.StatusCode != 403 {
			return fmt.Errorf("cross-origin location upgrade accepted")
		}
		response.Body.Close()
		rejected.Add(1)
		c, response, err = a.dial(fmt.Sprintf("/dashboard/%d/", other.ID), a.f.base)
		if err != nil {
			if response != nil {
				response.Body.Close()
			}
			return err
		}
		defer c.Close()
		c.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, _, err = c.ReadMessage()
		if _, ok := err.(*socket.CloseError); !ok {
			return fmt.Errorf("alien dashboard was not explicitly closed: %v", err)
		}
		rejected.Add(1)
		return nil
	})
	for i, a := range actors {
		u, err := us.GetByID(a.ID)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("Actualizado %04d", i), u.Name)
		require.Equal(t, a.Email, u.Email)
		require.Equal(t, a.Role, u.Role, "mass assignment changed role")
		h, err := hs.GetByID(a.PersonalID)
		require.NoError(t, err)
		require.NotNil(t, h.RecipientID)
		require.Equal(t, actors[(i+1)%users].ID, *h.RecipientID)
	}
	phase("moderator_and_administrator_workflows", func(i int, a *simulatedActor) error { return exerciseRoleWorkflow(i, a, actors, deviceIDs) })

	phase("gps_delivery_forgery_and_revocation", func(i int, a *simulatedActor) error {
		receiver := actors[(i+1)%users]
		own, err := a.dashboard()
		if err != nil {
			return err
		}
		defer own.Close()
		if _, err = readLoadMessage(own, time.Now().Add(12*time.Second)); err != nil {
			return err
		}
		view, err := receiver.dashboard()
		if err != nil {
			return err
		}
		defer view.Close()
		if _, err = readLoadMessage(view, time.Now().Add(12*time.Second)); err != nil {
			return err
		}
		producer, response, err := a.dial("/location", a.f.base)
		if err != nil {
			if response != nil {
				response.Body.Close()
			}
			return err
		}
		defer producer.Close()
		lat := 10 + float64(i)/1000
		err = producer.WriteJSON(map[string]any{"latitude": lat, "longitude": -80.5, "halcon_id": receiver.PersonalID, "user_id": receiver.ID})
		if err != nil {
			return err
		}
		if err = waitLoadCoordinate(own, a.PersonalID, a.ID, lat); err != nil {
			return err
		}
		if err = waitLoadCoordinate(view, a.PersonalID, a.ID, lat); err != nil {
			return err
		}
		if err = producer.WriteJSON(map[string]any{"latitude": 999, "longitude": 0}); err != nil {
			return err
		}
		producer.SetReadDeadline(time.Now().Add(12 * time.Second))
		_, _, err = producer.ReadMessage()
		if _, ok := err.(*socket.CloseError); !ok {
			return fmt.Errorf("invalid GPS not rejected: %v", err)
		}
		rejected.Add(1)
		r, err := a.post("/api/tracking/recipient", url.Values{"recipient_id": {"0"}})
		if err = loadStatus(r, err, 200); err != nil {
			return err
		}
		if receiver.Role != "admin" {
			if err = waitLoadRevocation(view, a.PersonalID); err != nil {
				return err
			}
		}
		saved, err := hs.GetByID(a.PersonalID)
		if err != nil {
			return err
		}
		if math.Abs(saved.LastLat-lat) > 0.0000001 || saved.LastLng != -80.5 || saved.OwnerID == nil || *saved.OwnerID != a.ID || saved.RecipientID != nil {
			return fmt.Errorf("GPS identity, precision or revocation mismatch")
		}
		return nil
	})

	// Hold 1000 authenticated viewers at once. Every frame is checked for
	// tenant leaks during a real broadcast, not just the initial snapshot.
	viewerStart := time.Now()
	sockets := make([]*socket.Conn, users)
	phase("open_1000_simultaneous_viewers", func(i int, a *simulatedActor) error {
		c, err := a.dashboard()
		if err != nil {
			return err
		}
		sockets[i] = c
		m, err := readLoadMessage(c, time.Now().Add(20*time.Second))
		if err != nil {
			return err
		}
		if m.Type != "snapshot" {
			return fmt.Errorf("missing initial snapshot")
		}
		return nil
	})
	defer func() {
		for _, c := range sockets {
			if c != nil {
				c.Close()
			}
		}
	}()
	for _, c := range sockets {
		require.NotNil(t, c)
	}
	r, err := actors[25].post("/api/tracking/recipient", url.Values{"recipient_id": {fmt.Sprint(actors[26].ID)}})
	require.NoError(t, loadStatus(r, err, 200))
	producer, response, err := actors[25].dial("/location", f.base)
	if response != nil && err != nil {
		response.Body.Close()
	}
	require.NoError(t, err)
	defer producer.Close()
	var wg sync.WaitGroup
	leaks := make(chan string, users)
	var deliveries atomic.Int64
	for i, c := range sockets {
		wg.Add(1)
		go func(i int, c *socket.Conn) {
			defer wg.Done()
			allowed := i == 25 || i == 26 || i < 2
			received := false
			deadline := time.Now().Add(6 * time.Second)
			for {
				m, e := readLoadMessage(c, deadline)
				if e != nil {
					if timeout, ok := e.(net.Error); !ok || !timeout.Timeout() {
						leaks <- fmt.Sprintf("viewer %d read failed: %v", i, e)
					}
					break
				}
				if m.Type == "" && m.ID == actors[25].PersonalID {
					if !allowed {
						leaks <- fmt.Sprintf("GPS leaked to viewer %d", i)
						return
					}
					if m.Lat == 55.25 {
						received = true
					}
				}
				if m.Type == "snapshot" {
					for _, h := range m.Halcones {
						if h.ID == actors[25].PersonalID && !allowed {
							leaks <- fmt.Sprintf("snapshot leaked to viewer %d", i)
							return
						}
					}
				}
			}
			if allowed && !received {
				leaks <- fmt.Sprintf("authorized viewer %d missed broadcast", i)
			} else if received {
				deliveries.Add(1)
			}
		}(i, c)
	}
	require.NoError(t, producer.WriteJSON(map[string]any{"latitude": 55.25, "longitude": -75.75}))
	wg.Wait()
	close(leaks)
	for leak := range leaks {
		failures = append(failures, leak)
	}
	require.Empty(t, failures)
	require.Equal(t, int64(4), deliveries.Load())
	rejected.Add(996)
	for _, c := range sockets {
		c.Close()
	}
	producer.Close()
	phases["1000_simultaneous_viewers_seconds"] = time.Since(viewerStart).Seconds()
	r, err = actors[25].post("/api/tracking/recipient", url.Values{"recipient_id": {"0"}})
	require.NoError(t, loadStatus(r, err, 200))
	for i, id := range deviceIDs {
		r, err := actors[i+2].post(fmt.Sprintf("/moderator/halcones/%d/delete", id), url.Values{})
		require.NoError(t, loadRedirect(r, err))
		_, err = hs.GetByID(id)
		require.Error(t, err)
	}
	phase("logout_and_session_revocation", func(i int, a *simulatedActor) error {
		path := "/api/auth/logout"
		if i%2 == 0 {
			path = "/logout"
		}
		r, err := a.post(path, url.Values{})
		if i%2 == 0 {
			err = loadRedirect(r, err)
		} else {
			err = loadStatus(r, err, 204)
		}
		if err != nil {
			return err
		}
		r, err = a.request("GET", "/api/tracking", nil)
		if err = loadStatus(r, err, 401); err != nil {
			return err
		}
		rejected.Add(1)
		c, response, err := a.dial("/location", f.base)
		if c != nil {
			c.Close()
		}
		if err == nil || response == nil || response.StatusCode != 401 {
			return fmt.Errorf("logged-out GPS upgrade accepted")
		}
		response.Body.Close()
		rejected.Add(1)
		return nil
	})
	count, err := facades.Orm().Query().Model(&models.User{}).Count()
	require.NoError(t, err)
	require.Equal(t, int64(users), count)
	count, err = facades.Orm().Query().Model(&models.Halcon{}).Where("owner_id IS NOT NULL").Count()
	require.NoError(t, err)
	require.Equal(t, int64(users), count)
	t.Logf("1000 users completed; 1000 simultaneous viewers; %d rejected access assertions", rejected.Load())
}
