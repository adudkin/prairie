package game

import "sync"

// Storage aggregates all world data.
type Storage struct {
	mu sync.Mutex

	// World tick
	tick int64

	// Entities
	locations [][]Location
	items     map[int]Item
	chars     map[int]*Character
	stations  map[int]Station
	vehicles  map[int]Vehicle

	// IDs
	nextItemID    int
	nextCharID    int
	nextStationID int
	nextVehicleID int

	// Player ID
	playerID int

	// Counters for NPC naming
	nextNPCNumber int
}

func NewStorage() *Storage {
	return &Storage{
		items:         make(map[int]Item),
		chars:         make(map[int]*Character),
		stations:      make(map[int]Station),
		vehicles:      make(map[int]Vehicle),
		nextItemID:    1,
		nextCharID:    1,
		nextStationID: 1,
		nextVehicleID: 1,
		nextNPCNumber: 1,
	}
}

func (s *Storage) WithLock(fn func()) { s.mu.Lock(); defer s.mu.Unlock(); fn() }

func (s *Storage) Tick() int64                 { return s.tick }
func (s *Storage) IncTick()                    { s.tick++ }
func (s *Storage) SetGrid(w, h int) {
	s.locations = make([][]Location, h)
	for y := 0; y < h; y++ {
		s.locations[y] = make([]Location, w)
		for x := 0; x < w; x++ {
			s.locations[y][x] = Location{X: x, Y: y, Name: LocName(x, y)}
		}
	}
}
func (s *Storage) InBounds(x, y int) bool {
	return x >= 0 && x < W && y >= 0 && y < H
}
func (s *Storage) LocationName(x, y int) string {
	if !s.InBounds(x, y) { return "" }
	return s.locations[y][x].Name
}

func (s *Storage) CreatePlayer(x, y int) int {
	id := s.nextCharID; s.nextCharID++
	s.chars[id] = &Character{ID: id, Name: "you", X: x, Y: y, Energy: 100, IsPlayer: true}
	s.playerID = id
	return id
}

func (s *Storage) Player() *Character { return s.chars[s.playerID] }

func (s *Storage) SpawnGreen(x, y int, baseName string) int {
	id := s.nextCharID; s.nextCharID++
	name := baseName
	if baseName == "auto-number" {
		name = "green character#" + strconvI(s.nextNPCNumber)
		s.nextNPCNumber++
	}
	s.chars[id] = &Character{ID: id, Name: name, X: x, Y: y, HP: 50, IsNPC: true}
	return id
}

func (s *Storage) RemoveChar(id int) {
	delete(s.chars, id)
}

func (s *Storage) NPCsHere(x, y int) []Character {
	out := []Character{}
	for _, c := range s.chars {
		if c.IsNPC && c.X == x && c.Y == y {
			out = append(out, *c)
		}
	}
	return out
}

func (s *Storage) Window3x3(x, y int) (names [][]string, counts [][]int) {
	names = make([][]string, 3)
	counts = make([][]int, 3)
	for i := 0; i < 3; i++ {
		names[i] = make([]string, 3)
		counts[i] = make([]int, 3)
	}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			nx, ny := x+dx, y+dy
			if s.InBounds(nx, ny) {
				names[dy+1][dx+1] = s.LocationName(nx, ny)
				counts[dy+1][dx+1] = s.countNPCsAt(nx, ny)
			} else {
				names[dy+1][dx+1] = ""
				counts[dy+1][dx+1] = 0
			}
		}
	}
	return
}

func (s *Storage) countNPCsAt(x, y int) int {
	c := 0
	for _, n := range s.chars {
		if n.IsNPC && n.X == x && n.Y == y { c++ }
	}
	return c
}

func strconvI(i int) string {
	// tiny helper to keep this file import-light
	if i == 0 { return "0" }
	sign := ""
	if i < 0 { sign = "-"; i = -i }
	d := []byte{}
	for i > 0 {
		d = append([]byte{byte('0' + i%10)}, d...)
		i /= 10
	}
	return sign + string(d)
}
