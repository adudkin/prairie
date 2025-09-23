package game

// InitWorld builds the grid, player, and initial NPCs.
func InitWorld(st *Storage, lg *Logger) {
	st.WithLock(func() {
		if !st.HasGrid() {
			st.SetGrid(W, H)
		}
		if st.HasPlayer() {
			return
		}
		st.CreatePlayer(0, 0)
		// Initial unnumbered green character at (5,7)
		id := st.SpawnGreen(5, 7, "green character")
		_ = id
		lg.Add(LogEntry{Tick: st.Tick(), Component: "spawn", Message: "spawned green character at " + LocName(5, 7) + " (HP 50)"})
	})
}
