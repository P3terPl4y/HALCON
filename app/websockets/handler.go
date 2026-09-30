package websocket

import (
	"encoding/json"
	"github.com/gofiber/contrib/v3/websocket"
	"goravel/app/services"
	"strconv"
	"time"
)

type locationPayload struct {
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

func decodeLocation(data []byte) (Coordinate, bool) {
	var p locationPayload
	if json.Unmarshal(data, &p) != nil || p.Latitude == nil || p.Longitude == nil {
		return Coordinate{}, false
	}
	lat, lng := *p.Latitude, *p.Longitude
	if !services.ValidCoordinates(lat, lng) {
		return Coordinate{}, false
	}
	return Coordinate{Latitude: lat, Longitude: lng}, true
}

func sessionValid(c *websocket.Conn) bool {
	check, ok := c.Locals("session_valid").(func() bool)
	expires, _ := c.Locals("session_expires").(time.Time)
	return time.Now().Before(expires) && ok && check()
}

func HandleHalcon(hub *Hub, service *services.HalconService) func(*websocket.Conn) {
	return func(c *websocket.Conn) {
		defer c.Close()
		h, err := service.ValidateToken(c.Query("token"))
		if err != nil || h.OwnerID != nil {
			return
		} // Personal producers require a live session.
		id, err := strconv.ParseUint(c.Params("id_halcon"), 10, strconv.IntSize)
		if err != nil || uint(id) != h.ID {
			return
		}
		routeUser, err := strconv.ParseUint(c.Params("id_user"), 10, strconv.IntSize)
		if err != nil {
			return
		}
		assignment, err := service.GetActiveAssignment(h.ID)
		if err != nil || assignment.UserID != uint(routeUser) {
			return
		}
		hc := &HalconConnection{Conn: c, HalconID: h.ID, UserID: assignment.UserID}
		produce(hub, service, hc, func() bool {
			current, err := service.ValidateToken(c.Query("token"))
			if err != nil || current.ID != h.ID {
				return false
			}
			a, err := service.GetActiveAssignment(h.ID)
			if err != nil || a.UserID != hc.UserID {
				return false
			}
			u, err := services.NewUserService().GetByID(a.UserID)
			return err == nil && u.Status
		})
	}
}
func HandlePersonal(hub *Hub, service *services.HalconService) func(*websocket.Conn) {
	return func(c *websocket.Conn) {
		defer c.Close()
		uid, ok := c.Locals("user_id").(uint)
		if !ok {
			return
		}
		h, err := service.EnsurePersonal(uid)
		if err != nil {
			return
		}
		produce(hub, service, &HalconConnection{Conn: c, HalconID: h.ID, UserID: uid}, func() bool {
			u, err := services.NewUserService().GetByID(uid)
			current, herr := service.GetByID(h.ID)
			return err == nil && herr == nil && u.Status && current.OwnerID != nil && *current.OwnerID == uid && sessionValid(c)
		})
	}
}
func produce(hub *Hub, service *services.HalconService, hc *HalconConnection, authorized func() bool) {
	hub.RegisterHalcon(hc)
	defer hub.UnregisterHalcon(hc, func() error { return service.Deactivate(hc.HalconID) })
	hc.Conn.SetReadLimit(4096)
	var lastAccepted time.Time
	for {
		_ = hc.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		mt, data, err := hc.Conn.ReadMessage()
		if err != nil {
			return
		}
		if mt != websocket.TextMessage {
			continue
		}
		coord, ok := decodeLocation(data)
		if !ok {
			return
		}
		if !authorized() {
			_ = hc.Conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "Autorización revocada"), time.Now().Add(time.Second))
			return
		}
		if time.Since(lastAccepted) < 250*time.Millisecond {
			continue
		}
		lastAccepted = time.Now()
		coord.HalconID, coord.UserID = hc.HalconID, hc.UserID
		err = hub.Publish(hc, coord, func() error { return service.UpdateLastLocation(hc.HalconID, coord.Latitude, coord.Longitude) })
		if err != nil {
			return
		}
	}
}

func HandleDashboard(hub *Hub, service *services.HalconService) func(*websocket.Conn) {
	return func(c *websocket.Conn) {
		defer c.Close()
		uid, ok := c.Locals("user_id").(uint)
		if !ok {
			return
		}
		id, err := strconv.ParseUint(c.Params("id_user"), 10, strconv.IntSize)
		if err != nil || uint(id) != uid {
			return
		}
		dc := &DashboardConnection{Conn: c, UserID: uid, Updates: make(chan Coordinate, 64)}
		hub.RegisterDashboard(dc)
		defer hub.UnregisterDashboard(dc)
		done := make(chan struct{})
		c.SetReadLimit(1024)
		go func() {
			defer close(done)
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
			}
		}()
		defer func() { _ = c.Close(); <-done }()
		write := func(v any) error { _ = c.SetWriteDeadline(time.Now().Add(5 * time.Second)); return c.WriteJSON(v) }
		snapshot := func() bool {
			hs, err := service.VisibleTo(uid)
			if err != nil {
				return false
			}
			states := make([]map[string]any, 0, len(hs))
			for _, h := range hs {
				active := h.IsActive && h.LastSeen != nil && time.Since(*h.LastSeen) < 60*time.Second
				states = append(states, map[string]any{"type": "halcon_status", "halcon_id": h.ID, "name": h.Name, "active": active, "lat": h.LastLat, "lng": h.LastLng, "has_location": h.LastSeen != nil})
			}
			return write(map[string]any{"type": "snapshot", "halcones": states}) == nil
		}
		if !snapshot() {
			return
		}
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if !sessionValid(c) || !snapshot() {
					return
				}
			case coord := <-dc.Updates:
				if !sessionValid(c) {
					return
				}
				allowed, err := service.CanView(coord.HalconID, uid)
				if err != nil {
					return
				}
				if allowed {
					if err := write(coord); err != nil {
						return
					}
				}
			}
		}
	}
}
