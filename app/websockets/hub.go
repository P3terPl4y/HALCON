package websocket

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gofiber/contrib/v3/websocket"
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
	Conn   *websocket.Conn
	UserID uint
}

type Hub struct {
	halconesActivos map[uint]*HalconConnection
	dashboards      map[uint][]*DashboardConnection
	mu              sync.RWMutex

	registerHalcon   chan *HalconConnection
	unregisterHalcon chan uint
	registerDash     chan *DashboardConnection
	unregisterDash   chan *DashboardConnection
	broadcast        chan Coordinate
}

func NewHub() *Hub {
	return &Hub{
		halconesActivos:  make(map[uint]*HalconConnection),
		dashboards:       make(map[uint][]*DashboardConnection),
		registerHalcon:   make(chan *HalconConnection),
		unregisterHalcon: make(chan uint),
		registerDash:     make(chan *DashboardConnection),
		unregisterDash:   make(chan *DashboardConnection),
		broadcast:        make(chan Coordinate, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case hc := <-h.registerHalcon:
			h.mu.Lock()
			if existing, ok := h.halconesActivos[hc.HalconID]; ok {
				existing.Conn.Close()
			}
			h.halconesActivos[hc.HalconID] = hc
			h.mu.Unlock()
			log.Printf("WS: halcón %d conectado (user %d)", hc.HalconID, hc.UserID)

		case halconID := <-h.unregisterHalcon:
			h.mu.Lock()
			if hc, ok := h.halconesActivos[halconID]; ok {
				delete(h.halconesActivos, halconID)
				hc.Conn.Close()
			}
			h.mu.Unlock()

		case dc := <-h.registerDash:
			h.mu.Lock()
			h.dashboards[dc.UserID] = append(h.dashboards[dc.UserID], dc)
			h.mu.Unlock()

		case dc := <-h.unregisterDash:
			h.mu.Lock()
			conns := h.dashboards[dc.UserID]
			for i, c := range conns {
				if c == dc {
					h.dashboards[dc.UserID] = append(conns[:i], conns[i+1:]...)
					break
				}
			}
			h.mu.Unlock()

		case coord := <-h.broadcast:
			h.mu.RLock()
			for _, dc := range h.dashboards[coord.UserID] {
				data, _ := json.Marshal(coord)
				if err := dc.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
					log.Printf("WS: error al escribir: %v", err)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) RegisterHalcon(hc *HalconConnection)         { h.registerHalcon <- hc }
func (h *Hub) UnregisterHalcon(halconID uint)              { h.unregisterHalcon <- halconID }
func (h *Hub) RegisterDashboard(dc *DashboardConnection)   { h.registerDash <- dc }
func (h *Hub) UnregisterDashboard(dc *DashboardConnection) { h.unregisterDash <- dc }
func (h *Hub) BroadcastCoordinate(coord Coordinate)        { h.broadcast <- coord }
