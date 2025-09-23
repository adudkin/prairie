package game

import "strconv"

// ===== Basic entities =====

type Location struct {
	X, Y  int
	Name  string
}

type Item struct {
	ID   int
	Name string
	// add more fields as needed (rarity, type, etc.)
}

type Character struct {
	ID       int
	Name     string
	X, Y     int
	HP       int // 0..100 for NPCs; player can have HP later if you wish
	Energy   int // 0..100 (player only currently used)
	IsPlayer bool
	IsNPC    bool
}

type Station struct {
	ID   int
	Name string
	X, Y int
}

type Vehicle struct {
	ID   int
	Name string
	X, Y int
}

// ===== Logging =====

type LogEntry struct {
	Tick      int64  `json:"tick"`
	Component string `json:"component"`
	Message   string `json:"message"`
}

// ===== State for frontend =====

type ActionLive struct {
	Active    bool   `json:"active"`
	Name      string `json:"name"`      // "moving" | "attack"
	Total     int64  `json:"total"`
	Remaining int64  `json:"remaining"`

	// moving
	DestX int    `json:"destX,omitempty"`
	DestY int    `json:"destY,omitempty"`
	Dest  string `json:"dest,omitempty"`

	// attack
	TargetID   int    `json:"targetId,omitempty"`
	TargetName string `json:"targetName,omitempty"`
	LastResult string `json:"lastResult,omitempty"` // "pending" | "MISS" | "HIT 5"
}

type LastAction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	TargetID    int    `json:"targetId,omitempty"`
	TargetName  string `json:"targetName,omitempty"`
	CompletedAt int64  `json:"completedAt"`
}

type State struct {
	Tick            int64        `json:"tick"`
	X               int          `json:"x"`
	Y               int          `json:"y"`
	Energy          int          `json:"energy"`
	NextRegenIn     int64        `json:"nextRegenIn"`
	LocationName    string       `json:"locationName"`
	Window3x3       [][]string   `json:"window3x3"`
	Window3x3Counts [][]int      `json:"window3x3Counts"`
	Logs            []LogEntry   `json:"logs"`
	HereNPCs        []Character  `json:"hereNPCs"`
	Action          *ActionLive  `json:"action,omitempty"`
	LastAction      *LastAction  `json:"lastAction,omitempty"`
}

// ===== Helpers =====

func LocName(x, y int) string {
	return "(" + strconv.Itoa(x) + ":" + strconv.Itoa(y) + ")"
}
