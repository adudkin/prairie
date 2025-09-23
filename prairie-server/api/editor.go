package api

import (
	"embed"
	"encoding/json"
	"net/http"
	"strconv"

	"example.com/prairie/game"
)

//go:embed editor.html
var editorPage embed.FS

func (s *Server) handleEditorPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		s.writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := editorPage.ReadFile("editor.html")
	if err != nil {
		s.writeError(w, "editor not available", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) handleEditorData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		s.writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		Tick       int64            `json:"tick"`
		Locations  []game.Location  `json:"locations"`
		Characters []game.Character `json:"characters"`
	}
	s.st.WithLock(func() {
		payload.Tick = s.st.Tick()
		payload.Locations = append(payload.Locations, s.st.LocationsFlat()...)
		payload.Characters = append(payload.Characters, s.st.CharactersSnapshot()...)
	})
	s.writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleEditorLocation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		s.writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		X    int    `json:"x"`
		Y    int    `json:"y"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, "invalid request", http.StatusBadRequest)
		return
	}
	var (
		loc game.Location
		err error
	)
	s.st.WithLock(func() {
		loc, err = s.st.UpdateLocationName(req.X, req.Y, req.Name)
	})
	if err != nil {
		s.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.writeJSON(w, http.StatusOK, loc)
}

func (s *Server) handleEditorCharacter(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req struct {
			ID       int    `json:"id"`
			Name     string `json:"name"`
			X        int    `json:"x"`
			Y        int    `json:"y"`
			HP       int    `json:"hp"`
			Energy   int    `json:"energy"`
			IsPlayer bool   `json:"isPlayer"`
			IsNPC    bool   `json:"isNPC"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeError(w, "invalid request", http.StatusBadRequest)
			return
		}
		if req.HP < 0 {
			req.HP = 0
		}
		if req.Energy < 0 {
			req.Energy = 0
		}
		var (
			result game.Character
			err    error
		)
		s.st.WithLock(func() {
			if req.ID == 0 {
				result, err = s.st.CreateCharacter(game.Character{
					Name:     req.Name,
					X:        req.X,
					Y:        req.Y,
					HP:       req.HP,
					Energy:   req.Energy,
					IsPlayer: req.IsPlayer,
					IsNPC:    req.IsNPC,
				})
			} else {
				result, err = s.st.UpdateCharacter(game.Character{
					ID:       req.ID,
					Name:     req.Name,
					X:        req.X,
					Y:        req.Y,
					HP:       req.HP,
					Energy:   req.Energy,
					IsPlayer: req.IsPlayer,
					IsNPC:    req.IsNPC,
				})
			}
		})
		if err != nil {
			s.writeError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.writeJSON(w, http.StatusOK, result)
	case http.MethodDelete:
		id, err := strconv.Atoi(r.URL.Query().Get("id"))
		if err != nil || id <= 0 {
			s.writeError(w, "invalid id", http.StatusBadRequest)
			return
		}
		var opErr error
		s.st.WithLock(func() {
			opErr = s.st.DeleteCharacter(id)
		})
		if opErr != nil {
			s.writeError(w, opErr.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "POST, DELETE")
		s.writeError(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
