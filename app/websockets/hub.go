package websocket

import (
	"github.com/gofiber/contrib/v3/websocket"
	"sync"
	"time"
)

type Coordinate struct {
	HalconID  uint    `json:"halcon_id"`
	UserID    uint    `json:"user_id"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timestamp int64   `json:"timestamp,omitempty"`
}
type HalconConnection struct {
	Conn     *websocket.Conn
	HalconID uint
	UserID   uint
}
type DashboardConnection struct {
	Conn    *websocket.Conn
	UserID  uint
	Updates chan Coordinate
}
type Hub struct {
	mu              sync.Mutex
	halconesActivos map[uint]*HalconConnection
	dashboards      map[*DashboardConnection]struct{}
}

func NewHub() *Hub {
	return &Hub{halconesActivos: make(map[uint]*HalconConnection), dashboards: make(map[*DashboardConnection]struct{})}
}

func (h *Hub) RegisterHalcon(c *HalconConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old := h.halconesActivos[c.HalconID]; old != nil {
		_ = old.Conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4009, "Otra conexión está transmitiendo este halcón"), time.Now().Add(time.Second))
		_ = old.Conn.Close()
	}
	h.halconesActivos[c.HalconID] = c
}

// A replaced socket must never unregister or deactivate its successor.
func (h *Hub) UnregisterHalcon(c *HalconConnection, deactivate func() error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.halconesActivos[c.HalconID] == c {
		delete(h.halconesActivos, c.HalconID)
		_ = deactivate()
	}
	_ = c.Conn.Close()
}
func (h *Hub) Publish(c *HalconConnection, coord Coordinate, persist func() error) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.halconesActivos[c.HalconID] != c {
		return websocket.ErrCloseSent
	}
	if err := persist(); err != nil {
		return err
	}
	coord.Timestamp = time.Now().UnixMilli()
	for dc := range h.dashboards {
		select {
		case dc.Updates <- coord:
		default: /* Snapshot reconciles dropped updates. */
		}
	}
	return nil
}
func (h *Hub) RegisterDashboard(c *DashboardConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dashboards[c] = struct{}{}
}
func (h *Hub) UnregisterDashboard(c *DashboardConnection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.dashboards, c)
}
