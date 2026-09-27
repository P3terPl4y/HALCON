package websocket

import (
	"encoding/json"
	"log"
	"strconv"

	"goravel/app/services"

	"github.com/gofiber/contrib/v3/websocket"
	
)

func HandleHalcon(hub *Hub, halconService *services.HalconService) func(*websocket.Conn) {
	return func(c *websocket.Conn) {
		token := c.Query("token")
		if token == "" {
			log.Println("❌ WS halcón: token vacío en query")
			c.Close()
			return
		}
		log.Printf("🔑 WS halcón: token recibido (len=%d)", len(token))

		halcon, err := halconService.ValidateToken(token)
		if err != nil {
			log.Printf("❌ WS halcón: token inválido: %v", err)
			c.Close()
			return
		}
		log.Printf("✅ WS halcón: token válido, halcon_id=%d", halcon.ID)

		assignment, err := halconService.GetActiveAssignment(halcon.ID)
		if err != nil {
			log.Printf("❌ WS halcón %d: sin asignación activa: %v", halcon.ID, err)
			c.Close()
			return
		}
		log.Printf("✅ WS halcón %d: asignación activa user_id=%d (assignment_id=%d)",
			halcon.ID, assignment.UserID, assignment.ID)

		routeUserID, err := strconv.ParseUint(c.Params("id_user"), 10, 64)
		if err != nil {
			log.Printf("❌ WS halcón %d: id_user inválido en ruta: %v", halcon.ID, err)
			c.Close()
			return
		}
		log.Printf("📍 WS halcón %d: ruta id_user=%d, asignación user_id=%d",
			halcon.ID, routeUserID, assignment.UserID)

		if uint(routeUserID) != assignment.UserID {
			log.Printf("❌ WS halcón %d: MISMATCH — ruta=%d, asignación=%d",
				halcon.ID, routeUserID, assignment.UserID)
			c.Close()
			return
		}

		log.Printf("🎯 WS halcón %d: TODAS las validaciones OK, registrando en hub", halcon.ID)
		hc := &HalconConnection{Conn: c, HalconID: halcon.ID, UserID: assignment.UserID}
		hub.RegisterHalcon(hc)
		defer hub.UnregisterHalcon(halcon.ID)

		halconService.Activate(halcon.ID)
		defer halconService.Deactivate(halcon.ID)

		for {
			mt, msg, err := c.ReadMessage()
			if err != nil {
				log.Printf("WS halcón %d: lectura terminada: %v", halcon.ID, err)
				break
			}
			if mt != websocket.TextMessage {
				continue
			}

			var coord Coordinate
			if err := json.Unmarshal(msg, &coord); err != nil {
				log.Printf("WS halcón %d: JSON inválido: %v", halcon.ID, err)
				continue
			}
			if coord.Latitude < -90 || coord.Latitude > 90 || coord.Longitude < -180 || coord.Longitude > 180 {
				log.Printf("WS halcón %d: coordenada fuera de rango", halcon.ID)
				continue
			}

			coord.HalconID = halcon.ID
			coord.UserID = assignment.UserID

			halconService.UpdateLastLocation(halcon.ID, coord.Latitude, coord.Longitude)
			hub.BroadcastCoordinate(coord)
			log.Printf("📡 WS halcón %d: broadcast (%.6f, %.6f)", halcon.ID, coord.Latitude, coord.Longitude)
		}
	}
}
func HandleDashboard(hub *Hub, halconService *services.HalconService) func(*websocket.Conn) {
	return func(c *websocket.Conn) {
		log.Println("🔌 WS dashboard: conexión recibida")

		routeUserID, err := strconv.ParseUint(c.Params("id_user"), 10, 64)
		if err != nil {
			log.Printf("❌ WS dashboard: id_user inválido en ruta: %v", err)
			c.Close()
			return
		}
		userID := uint(routeUserID)

		// Leer el user_id que dejó el middleware en Locals.
		// c.Locals() sobre *websocket.Conn está documentado en el README de contrib.
		rawUID := c.Locals("user_id")
		
		if rawUID == nil {
			log.Println("❌ WS dashboard: Locals(user_id) vacío — middleware no corrió")
			c.Close()
			return
		}
		authUserID, ok := rawUID.(uint)
		if !ok {
			log.Printf("❌ WS dashboard: Locals(user_id) no es uint: %T", rawUID)
			c.Close()
			return
		}

		log.Printf("👤 WS dashboard: sesión=%d, ruta=%d", authUserID, userID)

		if authUserID != userID {
			log.Printf("❌ WS dashboard: MISMATCH sesión=%d ruta=%d", authUserID, userID)
			c.Close()
			return
		}

		log.Printf("🎯 WS dashboard: validación OK para user %d", userID)

		dc := &DashboardConnection{Conn: c, UserID: userID}
		hub.RegisterDashboard(dc)
		defer hub.UnregisterDashboard(dc)

		halcones, _ := halconService.GetByAssignedUserID(userID)
		log.Printf("📦 WS dashboard: user %d tiene %d halcones asignados", userID, len(halcones))

		for _, h := range halcones {
			initial := map[string]any{
				"type":      "halcon_status",
				"halcon_id": h.ID,
				"name":      h.Name,
				"active":    h.IsActive,
				"lat":       h.LastLat,
				"lng":       h.LastLng,
			}
			data, _ := json.Marshal(initial)
			if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Printf("WS dashboard: error al enviar estado inicial: %v", err)
			}
		}

		for {
			_, _, err := c.ReadMessage()
			if err != nil {
				log.Printf("WS dashboard: user %d desconectado: %v", userID, err)
				break
			}
		}
	}
}
