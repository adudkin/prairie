package game

import (
	"database/sql"
	"errors"
	"log"
	"strconv"
	"sync"

	_ "modernc.org/sqlite"
)

// Storage aggregates all world data and persists it in SQLite.
type Storage struct {
	mu sync.Mutex

	db *sql.DB

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

// NewStorage opens (or creates) a SQLite database at the provided path and loads
// the in-memory state from it. Passing an empty path creates a purely in-memory
// storage without persistence.
func NewStorage(dbPath string) (*Storage, error) {
	st := &Storage{}
	st.resetInMemory()

	if dbPath != "" {
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			return nil, err
		}
		st.db = db
		if err := st.migrate(); err != nil {
			return nil, err
		}
		if err := st.loadFromDB(); err != nil {
			return nil, err
		}
	}

	return st, nil
}

func (s *Storage) resetInMemory() {
	s.tick = 0
	s.locations = nil
	s.items = make(map[int]Item)
	s.chars = make(map[int]*Character)
	s.stations = make(map[int]Station)
	s.vehicles = make(map[int]Vehicle)
	s.nextItemID = 1
	s.nextCharID = 1
	s.nextStationID = 1
	s.nextVehicleID = 1
	s.playerID = 0
	s.nextNPCNumber = 1
}

func (s *Storage) migrate() error {
	if s.db == nil {
		return nil
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS locations (x INTEGER NOT NULL, y INTEGER NOT NULL, name TEXT NOT NULL, PRIMARY KEY (x, y));`,
		`CREATE TABLE IF NOT EXISTS items (id INTEGER PRIMARY KEY, name TEXT NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS characters (id INTEGER PRIMARY KEY, name TEXT NOT NULL, x INTEGER NOT NULL, y INTEGER NOT NULL, hp INTEGER NOT NULL, energy INTEGER NOT NULL, is_player INTEGER NOT NULL, is_npc INTEGER NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS stations (id INTEGER PRIMARY KEY, name TEXT NOT NULL, x INTEGER NOT NULL, y INTEGER NOT NULL);`,
		`CREATE TABLE IF NOT EXISTS vehicles (id INTEGER PRIMARY KEY, name TEXT NOT NULL, x INTEGER NOT NULL, y INTEGER NOT NULL);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) loadFromDB() error {
	if s.db == nil {
		return nil
	}

	meta := make(map[string]string)
	rows, err := s.db.Query(`SELECT key, value FROM metadata`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		meta[k] = v
	}
	if err := rows.Err(); err != nil {
		return err
	}

	s.tick = int64(atoiDefault(meta["tick"], 0))
	s.nextItemID = maxInt(1, atoiDefault(meta["next_item_id"], 1))
	s.nextCharID = maxInt(1, atoiDefault(meta["next_char_id"], 1))
	s.nextStationID = maxInt(1, atoiDefault(meta["next_station_id"], 1))
	s.nextVehicleID = maxInt(1, atoiDefault(meta["next_vehicle_id"], 1))
	s.playerID = atoiDefault(meta["player_id"], 0)
	s.nextNPCNumber = maxInt(1, atoiDefault(meta["next_npc_number"], 1))

	// Locations grid defaults to the generated names.
	s.locations = make([][]Location, H)
	for y := 0; y < H; y++ {
		s.locations[y] = make([]Location, W)
		for x := 0; x < W; x++ {
			s.locations[y][x] = Location{X: x, Y: y, Name: LocName(x, y)}
		}
	}
	locRows, err := s.db.Query(`SELECT x, y, name FROM locations`)
	if err != nil {
		return err
	}
	defer locRows.Close()
	for locRows.Next() {
		var x, y int
		var name string
		if err := locRows.Scan(&x, &y, &name); err != nil {
			return err
		}
		if s.InBounds(x, y) {
			s.locations[y][x].Name = name
		}
	}
	if err := locRows.Err(); err != nil {
		return err
	}

	itemRows, err := s.db.Query(`SELECT id, name FROM items`)
	if err != nil {
		return err
	}
	defer itemRows.Close()
	for itemRows.Next() {
		var id int
		var name string
		if err := itemRows.Scan(&id, &name); err != nil {
			return err
		}
		s.items[id] = Item{ID: id, Name: name}
	}
	if err := itemRows.Err(); err != nil {
		return err
	}

	charRows, err := s.db.Query(`SELECT id, name, x, y, hp, energy, is_player, is_npc FROM characters`)
	if err != nil {
		return err
	}
	defer charRows.Close()
	maxCharID := 0
	for charRows.Next() {
		var c Character
		var isPlayer, isNPC int
		if err := charRows.Scan(&c.ID, &c.Name, &c.X, &c.Y, &c.HP, &c.Energy, &isPlayer, &isNPC); err != nil {
			return err
		}
		c.IsPlayer = isPlayer != 0
		c.IsNPC = isNPC != 0
		ch := c
		s.chars[c.ID] = &ch
		if c.IsPlayer {
			s.playerID = c.ID
		}
		if c.ID > maxCharID {
			maxCharID = c.ID
		}
	}
	if err := charRows.Err(); err != nil {
		return err
	}
	if maxCharID >= s.nextCharID {
		s.nextCharID = maxCharID + 1
	}

	stationRows, err := s.db.Query(`SELECT id, name, x, y FROM stations`)
	if err != nil {
		return err
	}
	defer stationRows.Close()
	maxStationID := 0
	for stationRows.Next() {
		var st Station
		if err := stationRows.Scan(&st.ID, &st.Name, &st.X, &st.Y); err != nil {
			return err
		}
		s.stations[st.ID] = st
		if st.ID > maxStationID {
			maxStationID = st.ID
		}
	}
	if err := stationRows.Err(); err != nil {
		return err
	}
	if maxStationID >= s.nextStationID {
		s.nextStationID = maxStationID + 1
	}

	vehicleRows, err := s.db.Query(`SELECT id, name, x, y FROM vehicles`)
	if err != nil {
		return err
	}
	defer vehicleRows.Close()
	maxVehicleID := 0
	for vehicleRows.Next() {
		var v Vehicle
		if err := vehicleRows.Scan(&v.ID, &v.Name, &v.X, &v.Y); err != nil {
			return err
		}
		s.vehicles[v.ID] = v
		if v.ID > maxVehicleID {
			maxVehicleID = v.ID
		}
	}
	if err := vehicleRows.Err(); err != nil {
		return err
	}
	if maxVehicleID >= s.nextVehicleID {
		s.nextVehicleID = maxVehicleID + 1
	}

	return nil
}

func atoiDefault(v string, def int) int {
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (s *Storage) WithLock(fn func()) { s.mu.Lock(); defer s.mu.Unlock(); fn() }

func (s *Storage) Tick() int64 { return s.tick }

func (s *Storage) IncTick() {
	s.tick++
	s.setMetaInt64("tick", s.tick)
}

func (s *Storage) SetGrid(w, h int) {
	s.locations = make([][]Location, h)
	for y := 0; y < h; y++ {
		s.locations[y] = make([]Location, w)
		for x := 0; x < w; x++ {
			s.locations[y][x] = Location{X: x, Y: y, Name: LocName(x, y)}
			s.persistLocation(x, y, s.locations[y][x].Name)
		}
	}
}

func (s *Storage) HasGrid() bool {
	return len(s.locations) == H && (H == 0 || len(s.locations[0]) == W)
}

func (s *Storage) InBounds(x, y int) bool {
	return x >= 0 && x < W && y >= 0 && y < H
}

func (s *Storage) LocationName(x, y int) string {
	if !s.InBounds(x, y) {
		return ""
	}
	return s.locations[y][x].Name
}

func (s *Storage) CreatePlayer(x, y int) int {
	id := s.allocateCharID()
	s.chars[id] = &Character{ID: id, Name: "you", X: x, Y: y, Energy: 100, HP: 100, IsPlayer: true}
	s.playerID = id
	s.saveCharacter(s.chars[id])
	s.setMetaInt("player_id", id)
	return id
}

func (s *Storage) HasPlayer() bool {
	if s.playerID == 0 {
		return false
	}
	_, ok := s.chars[s.playerID]
	return ok
}

func (s *Storage) Player() *Character { return s.chars[s.playerID] }

func (s *Storage) SpawnGreen(x, y int, baseName string) int {
	id := s.allocateCharID()
	name := baseName
	if baseName == "auto-number" {
		name = "green character#" + strconvI(s.nextNPCNumber)
		s.nextNPCNumber++
		s.setMetaInt("next_npc_number", s.nextNPCNumber)
	}
	s.chars[id] = &Character{ID: id, Name: name, X: x, Y: y, HP: 50, IsNPC: true}
	s.saveCharacter(s.chars[id])
	return id
}

func (s *Storage) RemoveChar(id int) {
	delete(s.chars, id)
	if s.db != nil {
		if _, err := s.db.Exec(`DELETE FROM characters WHERE id = ?`, id); err != nil {
			log.Printf("storage: failed to delete character %d: %v", id, err)
		}
	}
	if s.playerID == id {
		s.playerID = 0
		s.setMetaInt("player_id", 0)
	}
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
		if n.IsNPC && n.X == x && n.Y == y {
			c++
		}
	}
	return c
}

func (s *Storage) LocationsFlat() []Location {
	res := []Location{}
	for _, row := range s.locations {
		for _, loc := range row {
			res = append(res, loc)
		}
	}
	return res
}

func (s *Storage) UpdateLocationName(x, y int, name string) (Location, error) {
	if !s.InBounds(x, y) {
		return Location{}, errors.New("out of bounds")
	}
	if name == "" {
		name = LocName(x, y)
	}
	s.locations[y][x].Name = name
	if err := s.persistLocation(x, y, name); err != nil {
		return Location{}, err
	}
	return s.locations[y][x], nil
}

func (s *Storage) CharactersSnapshot() []Character {
	res := make([]Character, 0, len(s.chars))
	for _, c := range s.chars {
		res = append(res, *c)
	}
	return res
}

func (s *Storage) CreateCharacter(c Character) (Character, error) {
	if c.IsPlayer && s.playerID != 0 {
		return Character{}, errors.New("player already exists")
	}
	id := s.allocateCharID()
	c.ID = id
	ch := &Character{
		ID:       c.ID,
		Name:     c.Name,
		X:        c.X,
		Y:        c.Y,
		HP:       c.HP,
		Energy:   c.Energy,
		IsPlayer: c.IsPlayer,
		IsNPC:    c.IsNPC,
	}
	if ch.HP == 0 && ch.IsNPC {
		ch.HP = 50
	}
	s.chars[id] = ch
	if ch.IsPlayer {
		s.playerID = ch.ID
		s.setMetaInt("player_id", ch.ID)
	}
	s.saveCharacter(ch)
	return *ch, nil
}

func (s *Storage) UpdateCharacter(c Character) (Character, error) {
	existing, ok := s.chars[c.ID]
	if !ok {
		return Character{}, errors.New("character not found")
	}
	if c.IsPlayer && s.playerID != 0 && s.playerID != c.ID {
		return Character{}, errors.New("another player already exists")
	}
	existing.Name = c.Name
	existing.X = c.X
	existing.Y = c.Y
	existing.HP = c.HP
	existing.Energy = c.Energy
	existing.IsNPC = c.IsNPC
	existing.IsPlayer = c.IsPlayer
	if existing.IsPlayer {
		s.playerID = existing.ID
		s.setMetaInt("player_id", existing.ID)
	} else if s.playerID == existing.ID {
		s.playerID = 0
		s.setMetaInt("player_id", 0)
	}
	s.saveCharacter(existing)
	return *existing, nil
}

func (s *Storage) DeleteCharacter(id int) error {
	if _, ok := s.chars[id]; !ok {
		return errors.New("character not found")
	}
	delete(s.chars, id)
	if s.db != nil {
		if _, err := s.db.Exec(`DELETE FROM characters WHERE id = ?`, id); err != nil {
			return err
		}
	}
	if s.playerID == id {
		s.playerID = 0
		s.setMetaInt("player_id", 0)
	}
	return nil
}

func (s *Storage) allocateCharID() int {
	id := s.nextCharID
	s.nextCharID++
	s.setMetaInt("next_char_id", s.nextCharID)
	return id
}

func (s *Storage) persistLocation(x, y int, name string) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.Exec(`INSERT INTO locations (x, y, name) VALUES (?, ?, ?) `+
		`ON CONFLICT(x, y) DO UPDATE SET name = excluded.name`, x, y, name)
	if err != nil {
		log.Printf("storage: failed to persist location (%d,%d): %v", x, y, err)
	}
	return err
}

func (s *Storage) saveCharacter(c *Character) {
	if s.db == nil {
		return
	}
	_, err := s.db.Exec(`INSERT INTO characters (id, name, x, y, hp, energy, is_player, is_npc) VALUES (?, ?, ?, ?, ?, ?, ?, ?) `+
		`ON CONFLICT(id) DO UPDATE SET name = excluded.name, x = excluded.x, y = excluded.y, hp = excluded.hp, energy = excluded.energy, is_player = excluded.is_player, is_npc = excluded.is_npc`,
		c.ID, c.Name, c.X, c.Y, c.HP, c.Energy, boolToInt(c.IsPlayer), boolToInt(c.IsNPC))
	if err != nil {
		log.Printf("storage: failed to persist character %d: %v", c.ID, err)
	}
}

func (s *Storage) setMetaInt(key string, value int) {
	if s.db == nil {
		return
	}
	_, err := s.db.Exec(`INSERT INTO metadata (key, value) VALUES (?, ?) `+
		`ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, strconv.Itoa(value))
	if err != nil {
		log.Printf("storage: failed to update metadata %s: %v", key, err)
	}
}

func (s *Storage) setMetaInt64(key string, value int64) {
	if s.db == nil {
		return
	}
	_, err := s.db.Exec(`INSERT INTO metadata (key, value) VALUES (?, ?) `+
		`ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, strconv.FormatInt(value, 10))
	if err != nil {
		log.Printf("storage: failed to update metadata %s: %v", key, err)
	}
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func strconvI(i int) string {
	// tiny helper to keep this file import-light
	if i == 0 {
		return "0"
	}
	sign := ""
	if i < 0 {
		sign = "-"
		i = -i
	}
	d := []byte{}
	for i > 0 {
		d = append([]byte{byte('0' + i%10)}, d...)
		i /= 10
	}
	return sign + string(d)
}
