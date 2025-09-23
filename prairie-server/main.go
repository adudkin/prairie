package main

import (
	"log"
	"time"

	"example.com/prairie/api"
	"example.com/prairie/game"
)

func main() {
	// Core singletons
	st, err := game.NewStorage("prairie.db")
	if err != nil {
		log.Fatalf("failed to initialize storage: %v", err)
	}
	lg := game.NewLogger(800)
	pr := game.NewProcessor(st, lg)

	// Build the world (10x10, initial NPC at (5,7), etc.)
	game.InitWorld(st, lg)

	// Ticker: each second enqueue a Tick action, then let Processor advance 1 tick
	t := time.NewTicker(1 * time.Second)
	defer t.Stop()

	// HTTP API (wires handlers and starts the server)
	srv := api.NewServer(st, pr, lg)

	go func() {
		for range t.C {
			// Every real-time second is one simulation tick:
			// 1) Enqueue a tick-refresh action
			pr.Enqueue(game.Action{Kind: game.ActionTick})
			// 2) Let the processor apply one-tick worth of progress
			pr.OnNewTick()
		}
	}()

	log.Printf("prairie server listening on %s", api.ServerPort)
	log.Fatal(srv.ListenAndServe())
}
