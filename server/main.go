package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ═══════════════════════════════════════════
//  DATA TYPES
// ═══════════════════════════════════════════

type MsgType string

const (
	// Client -> Server
	MsgJoin       MsgType = "join"
	MsgBoardState MsgType = "board_state"
	MsgAttack     MsgType = "attack"
	MsgGameOver   MsgType = "game_over"
	MsgLeave      MsgType = "leave"
	MsgReady      MsgType = "ready"

	// Server -> Client
	MsgWaiting     MsgType = "waiting"
	MsgMatchFound  MsgType = "match_found"
	MsgOpBoard     MsgType = "opponent_board"
	MsgOpAttack    MsgType = "opponent_attack"
	MsgOpLeft      MsgType = "opponent_left"
	MsgOpReady     MsgType = "opponent_ready"
	MsgGameStart   MsgType = "game_start"
	MsgYouWin      MsgType = "you_win"
	MsgYouLose     MsgType = "you_lose"
	MsgError       MsgType = "error"
	MsgRoomInfo    MsgType = "room_info"
)

type Message struct {
	Type   MsgType         `json:"type"`
	Data   json.RawMessage `json:"data,omitempty"`
	RoomID string          `json:"room_id,omitempty"`
}

type JoinData struct {
	Name   string `json:"name"`
	RoomID string `json:"room_id,omitempty"` // empty = random match, non-empty = private room
}

type BoardStateData struct {
	Grid  [][]string `json:"grid"`  // 20x10, null or piece type
	Score int        `json:"score"`
	Lines int        `json:"lines"`
	Level int        `json:"level"`
}

type AttackData struct {
	Lines int `json:"lines"` // garbage lines to send
}

type MatchFoundData struct {
	Opponent string `json:"opponent"`
	RoomID   string `json:"room_id"`
	Side     string `json:"side"` // "left" or "right"
}

type RoomInfoData struct {
	RoomID  string `json:"room_id"`
	Players int    `json:"players"`
}

// ═══════════════════════════════════════════
//  PLAYER & ROOM
// ═══════════════════════════════════════════

type Player struct {
	conn   *websocket.Conn
	name   string
	room   *Room
	mu     sync.Mutex
	closed bool
}

func (p *Player) Send(msg Message) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := p.conn.WriteJSON(msg); err != nil {
		log.Printf("Send error to %s: %v", p.name, err)
	}
}

func (p *Player) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.closed {
		p.closed = true
		p.conn.Close()
	}
}

type Room struct {
	id       string
	players  [2]*Player
	count    int
	mu       sync.Mutex
	started  bool
	ready    [2]bool
	gameOn   bool
}

func (r *Room) Opponent(p *Player) *Player {
	if r.players[0] == p {
		return r.players[1]
	}
	return r.players[0]
}

// ═══════════════════════════════════════════
//  SERVER
// ═══════════════════════════════════════════

type Server struct {
	mu           sync.Mutex
	rooms        map[string]*Room
	waitingQueue *Player // single player waiting for random match
	upgrader     websocket.Upgrader
}

func NewServer() *Server {
	return &Server{
		rooms: make(map[string]*Room),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
		},
	}
}

func generateRoomID() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

func marshalData(v interface{}) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func (s *Server) handleJoin(p *Player, joinData JoinData) {
	p.name = joinData.Name
	if p.name == "" {
		p.name = fmt.Sprintf("Player_%d", rand.Intn(9999))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if joinData.RoomID != "" {
		// Private room
		roomID := joinData.RoomID
		room, exists := s.rooms[roomID]
		if !exists {
			// Create new private room
			room = &Room{id: roomID}
			s.rooms[roomID] = room
		}

		room.mu.Lock()
		if room.count >= 2 {
			room.mu.Unlock()
			p.Send(Message{Type: MsgError, Data: marshalData("Room is full")})
			return
		}
		room.players[room.count] = p
		room.count++
		p.room = room
		// Reset ready state for a fresh match
		room.ready[0] = false
		room.ready[1] = false
		room.gameOn = false

		if room.count == 2 {
			room.started = true
			room.mu.Unlock()
			// Match found!
			s.startMatch(room)
		} else {
			room.mu.Unlock()
			p.Send(Message{
				Type: MsgWaiting,
				Data: marshalData(RoomInfoData{RoomID: roomID, Players: 1}),
			})
		}
	} else {
		// Random matchmaking
		if s.waitingQueue != nil && s.waitingQueue != p {
			opponent := s.waitingQueue
			s.waitingQueue = nil

			roomID := generateRoomID()
			room := &Room{
				id:      roomID,
				players: [2]*Player{opponent, p},
				count:   2,
				started: true,
			}
			s.rooms[roomID] = room
			opponent.room = room
			p.room = room

			s.startMatch(room)
		} else {
			s.waitingQueue = p
			p.Send(Message{
				Type: MsgWaiting,
				Data: marshalData("Searching for opponent..."),
			})
		}
	}
}

func (s *Server) startMatch(room *Room) {
	log.Printf("Match started: %s vs %s in room %s",
		room.players[0].name, room.players[1].name, room.id)

	room.players[0].Send(Message{
		Type: MsgMatchFound,
		Data: marshalData(MatchFoundData{
			Opponent: room.players[1].name,
			RoomID:   room.id,
			Side:     "left",
		}),
	})
	room.players[1].Send(Message{
		Type: MsgMatchFound,
		Data: marshalData(MatchFoundData{
			Opponent: room.players[0].name,
			RoomID:   room.id,
			Side:     "right",
		}),
	})
}

func (s *Server) handleDisconnect(p *Player) {
	s.mu.Lock()
	if s.waitingQueue == p {
		s.waitingQueue = nil
	}
	s.mu.Unlock()

	if p.room != nil {
		room := p.room
		room.mu.Lock()
		op := room.Opponent(p)
		room.mu.Unlock()

		if op != nil {
			op.Send(Message{Type: MsgOpLeft})
		}

		s.mu.Lock()
		delete(s.rooms, room.id)
		s.mu.Unlock()
	}
	p.Close()
}

func (s *Server) handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade error: %v", err)
		return
	}

	player := &Player{conn: conn, name: "anonymous"}
	log.Printf("New connection from %s", conn.RemoteAddr())

	defer s.handleDisconnect(player)

	conn.SetReadDeadline(time.Now().Add(300 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(300 * time.Second))
		return nil
	})

	// Ping ticker - keep connection alive through proxies and load balancers
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			player.mu.Lock()
			if player.closed {
				player.mu.Unlock()
				return
			}
			player.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := player.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				player.mu.Unlock()
				return
			}
			player.mu.Unlock()
		}
	}()

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Read error from %s: %v", player.name, err)
			return
		}
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		var msg Message
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "ping":
			// Client heartbeat, just keeps the connection alive
			continue

		case MsgJoin:
			var data JoinData
			json.Unmarshal(msg.Data, &data)
			s.handleJoin(player, data)

		case MsgBoardState:
			if player.room == nil {
				continue
			}
			op := player.room.Opponent(player)
			if op != nil {
				op.Send(Message{Type: MsgOpBoard, Data: msg.Data})
			}

		case MsgAttack:
			if player.room == nil {
				continue
			}
			op := player.room.Opponent(player)
			if op != nil {
				op.Send(Message{Type: MsgOpAttack, Data: msg.Data})
			}

		case MsgGameOver:
			if player.room == nil {
				continue
			}
			op := player.room.Opponent(player)
			if op != nil {
				// The one who sent game_over lost
				player.Send(Message{Type: MsgYouLose})
				op.Send(Message{Type: MsgYouWin})
			}

		case MsgReady:
			if player.room == nil {
				continue
			}
			room := player.room
			room.mu.Lock()
			if room.count < 2 || room.gameOn {
				room.mu.Unlock()
				continue
			}
			idx := -1
			for i, rp := range room.players {
				if rp == player {
					idx = i
					break
				}
			}
			if idx < 0 {
				room.mu.Unlock()
				continue
			}
			alreadyReady := room.ready[idx]
			room.ready[idx] = true
			bothReady := room.ready[0] && room.ready[1]
			if bothReady {
				room.gameOn = true
			}
			p0 := room.players[0]
			p1 := room.players[1]
			room.mu.Unlock()

			if bothReady {
				p0.Send(Message{Type: MsgGameStart})
				p1.Send(Message{Type: MsgGameStart})
			} else if !alreadyReady {
				op := room.Opponent(player)
				if op != nil {
					op.Send(Message{Type: MsgOpReady})
				}
			}

		case MsgLeave:
			return
		}
	}
}

// ═══════════════════════════════════════════
//  MAIN
// ═══════════════════════════════════════════

func main() {
	rand.Seed(time.Now().UnixNano())
	server := NewServer()

	http.HandleFunc("/ws", server.handleConnection)

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(200)
		fmt.Fprintf(w, `{"status":"ok","rooms":%d}`, len(server.rooms))
	})

	// Simple landing page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><body style="background:#0a0a0f;color:#00ffaa;font-family:monospace;display:flex;justify-content:center;align-items:center;height:100vh;margin:0">
		<div style="text-align:center"><h1>TETRIS BATTLE SERVER</h1><p style="color:#888">WebSocket endpoint: ws://HOST/ws</p></div></body></html>`)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Tetris Battle Server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
