package game

import (
	"errors"
	"math/rand"
)

type Processor struct {
	st  *Storage
	log *Logger

	// queue + active
	queue  []Action
	active *ActionLive
	// last completed
	last *LastAction
}

func NewProcessor(st *Storage, logg *Logger) *Processor {
	return &Processor{st: st, log: logg, queue: []Action{}}
}

// Enqueue schedules an action to be started at the next tick if idle.
// (Per spec: "Every new tick queue is checked by Processor if being inactive at the moment of new tick")
func (p *Processor) Enqueue(a Action) {
	p.st.WithLock(func() {
		p.queue = append(p.queue, a)
	})
}

func (p *Processor) OnNewTick() {
	p.st.WithLock(func() {
		// 1) Apply the Tick "refresh" first (it also comes as an action)
		p.consumeAllLeadingTicks()

		// 2) If an action is active, decrement its remaining and complete if needed
		if p.active != nil && p.active.Active {
			if p.active.Remaining > 0 {
				p.active.Remaining--
			}
			if p.active.Remaining == 0 {
				p.completeActive()
			}
		}

		// 3) If now idle, try to start the next queued (non-tick) action
		if (p.active == nil || !p.active.Active) && len(p.queue) > 0 {
			// Peek; if it's a tick, it'll be consumed at the next real tick; start only non-tick
			if p.queue[0].Kind != ActionTick {
				_ = p.tryStart(p.queue[0])
				// pop regardless (if fails, it's a bad request from client; we drop it)
				p.queue = p.queue[1:]
			}
		}
	})
}

func (p *Processor) consumeAllLeadingTicks() {
	// process any leading tick actions immediately (they do not occupy the active slot)
	for len(p.queue) > 0 && p.queue[0].Kind == ActionTick {
		p.queue = p.queue[1:]
		p.applyTickRefresh()
	}
}

// === tick refresh: time passes, regen, spawns, bookkeeping
func (p *Processor) applyTickRefresh() {
	p.st.IncTick()
	t := p.st.Tick()

	// Energy regen
	player := p.st.Player()
	if player.Energy < MaxEnergy && t%RegenEvery == 0 {
		player.Energy++
		p.log.Add(LogEntry{Tick: t, Component: "energy", Message: "+1 (now " + strconvI(player.Energy) + ")"})
	}

	// Spawns
	if t%SpawnEvery == 0 {
		x := rand.Intn(W); y := rand.Intn(H)
		id := p.st.SpawnGreen(x, y, "auto-number")
		_ = id
		p.log.Add(LogEntry{Tick: t, Component: "spawn", Message: "spawned green at " + LocName(x, y) + " (HP 50)"})
	}
}

func (p *Processor) tryStart(a Action) error {
	player := p.st.Player()
	switch a.Kind {
	case ActionMove:
		nx, ny := player.X+a.DX, player.Y+a.DY
		if a.DX == 0 && a.DY == 0 { return errors.New("no move") }
		if !p.st.InBounds(nx, ny) { return errors.New("out of bounds") }
		if player.Energy <= 0 { return errors.New("not enough energy") }
		// spend energy at start
		player.Energy--
		p.active = &ActionLive{
			Active: true, Name: "moving", Total: ActionTicks, Remaining: ActionTicks,
			DestX: nx, DestY: ny, Dest: p.st.LocationName(nx, ny),
		}
	case ActionAttack:
		if player.Energy <= 0 { return errors.New("not enough energy") }
		// must target NPC in same tile
		var target *Character
		for _, c := range p.st.NPCsHere(player.X, player.Y) {
			if c.ID == a.TargetID { // Snapshot returns copies; find original
				target = p.st.chars[c.ID]
				break
			}
		}
		if target == nil { return errors.New("target not here") }
		player.Energy--
		p.active = &ActionLive{
			Active: true, Name: "attack", Total: ActionTicks, Remaining: ActionTicks,
			TargetID: target.ID, TargetName: target.Name, LastResult: "pending",
		}
	default:
	}
	return nil
}

func (p *Processor) completeActive() {
	if p.active == nil || !p.active.Active { return }
	player := p.st.Player()
	t := p.st.Tick()

	switch p.active.Name {
	case "moving":
		from := LocName(player.X, player.Y)
		player.X, player.Y = p.active.DestX, p.active.DestY
		p.log.Add(LogEntry{Tick: t, Component: "move",
			Message: "from " + from + " to " + LocName(player.X, player.Y) + " (energy now " + strconvI(player.Energy) + ")"})
		p.last = &LastAction{
			Name: "moving", Description: "to " + LocName(player.X, player.Y) + ", energy now " + strconvI(player.Energy),
			CompletedAt: t,
		}

	case "attack":
		// re-find target
		var target *Character
		for _, c := range p.st.NPCsHere(player.X, player.Y) {
			if c.ID == p.active.TargetID {
				target = p.st.chars[c.ID]
				break
			}
		}
		if target == nil {
			p.active.LastResult = "MISS"
			p.log.Add(LogEntry{Tick: t, Component: "combat",
				Message: "attack on " + p.active.TargetName + " missed (target not here) (energy now " + strconvI(player.Energy) + ")"})
			p.last = &LastAction{
				Name: "attack", Description: "attack on " + p.active.TargetName + ": MISS",
				TargetID: p.active.TargetID, TargetName: p.active.TargetName, CompletedAt: t,
			}
		} else {
			hit := rand.Float64() < HitChance
			if !hit {
				p.active.LastResult = "MISS"
				p.log.Add(LogEntry{Tick: t, Component: "combat",
					Message: "attack on " + p.active.TargetName + " missed (energy now " + strconvI(player.Energy) + ")"})
				p.last = &LastAction{
					Name: "attack", Description: "attack on " + p.active.TargetName + ": MISS",
					TargetID: p.active.TargetID, TargetName: p.active.TargetName, CompletedAt: t,
				}
			} else {
				target.HP -= DamageOnHit
				if target.HP < 0 { target.HP = 0 }
				p.active.LastResult = "HIT 5"
				p.log.Add(LogEntry{Tick: t, Component: "combat",
					Message: "attack on " + p.active.TargetName + " HIT for 5 (HP now " + strconvI(target.HP) + ", energy " + strconvI(player.Energy) + ")"})
				if target.HP <= 0 {
					p.log.Add(LogEntry{Tick: t, Component: "combat", Message: p.active.TargetName + " died"})
					p.st.RemoveChar(target.ID)
					p.last = &LastAction{
						Name: "attack", Description: "attack on " + p.active.TargetName + ": HIT 5 (dead)",
						TargetID: p.active.TargetID, TargetName: p.active.TargetName, CompletedAt: t,
					}
				} else {
					p.last = &LastAction{
						Name: "attack", Description: "attack on " + p.active.TargetName + ": HIT 5 (HP now " + strconvI(target.HP) + ")",
						TargetID: p.active.TargetID, TargetName: p.active.TargetName, CompletedAt: t,
					}
				}
			}
		}
	}
	p.active.Active = false
}

// ===== Query state for API =====

func (p *Processor) Snapshot() State {
	var st State
	p.st.WithLock(func() {
		pl := p.st.Player()
		names, counts := p.st.Window3x3(pl.X, pl.Y)
		logs := p.log.Snapshot()
		here := p.st.NPCsHere(pl.X, pl.Y)

		var act *ActionLive
		if p.active != nil && p.active.Active {
			a := *p.active
			act = &a
		}
		var last *LastAction
		if p.last != nil {
			l := *p.last
			last = &l
		}

		st = State{
			Tick:            p.st.Tick(),
			X:               pl.X,
			Y:               pl.Y,
			Energy:          pl.Energy,
			NextRegenIn:     p.nextRegenIn(pl),
			LocationName:    p.st.LocationName(pl.X, pl.Y),
			Window3x3:       names,
			Window3x3Counts: counts,
			Logs:            logs,
			HereNPCs:        here,
			Action:          act,
			LastAction:      last,
		}
	})
	return st
}

func (p *Processor) nextRegenIn(pl *Character) int64 {
	if pl.Energy >= MaxEnergy { return 0 }
	r := RegenEvery - (p.st.Tick() % RegenEvery)
	if r == RegenEvery { return 0 }
	return r
}
