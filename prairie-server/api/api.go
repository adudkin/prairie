package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"example.com/prairie/game"
)

const ServerPort = ":8080"

type Server struct {
	mux *http.ServeMux
	st  *game.Storage
	pr  *game.Processor
	lg  *game.Logger

	httpServer *http.Server
}

func NewServer(st *game.Storage, pr *game.Processor, lg *game.Logger) *Server {
	s := &Server{
		mux: http.NewServeMux(),
		st:  st,
		pr:  pr,
		lg:  lg,
	}
	s.routes()
	s.httpServer = &http.Server{
		Addr:    ServerPort,
		Handler: s.mux,
	}
	return s
}

func (s *Server) ListenAndServe() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) routes() {
        s.mux.HandleFunc("/editor", s.handleEditorPage)
        s.mux.HandleFunc("/editor/data", s.handleEditorData)
        s.mux.HandleFunc("/editor/location", s.handleEditorLocation)
        s.mux.HandleFunc("/editor/character", s.handleEditorCharacter)
        s.mux.HandleFunc("/state", s.handleState)
        s.mux.HandleFunc("/move", s.handleMove)
        s.mux.HandleFunc("/attack", s.handleAttack)
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) writeError(w http.ResponseWriter, msg string, code int) {
	http.Error(w, msg, code)
}

// GET /state
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	st := s.pr.Snapshot()
	s.writeJSON(w, http.StatusOK, st)
}

// POST /move?dx=&dy=
func (s *Server) handleMove(w http.ResponseWriter, r *http.Request) {
	dx, _ := strconv.Atoi(r.URL.Query().Get("dx"))
	dy, _ := strconv.Atoi(r.URL.Query().Get("dy"))
	// enqueue; Processor will start it at next tick if idle
	s.pr.Enqueue(game.Action{Kind: game.ActionMove, DX: dx, DY: dy})
	s.writeJSON(w, http.StatusOK, s.pr.Snapshot())
}

// POST /attack?targetId=
func (s *Server) handleAttack(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("targetId"))
	s.pr.Enqueue(game.Action{Kind: game.ActionAttack, TargetID: id})
	s.writeJSON(w, http.StatusOK, s.pr.Snapshot())
}
