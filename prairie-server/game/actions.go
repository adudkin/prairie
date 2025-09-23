package game

type ActionKind int

const (
	ActionTick ActionKind = iota
	ActionMove
	ActionAttack
)

type Action struct {
	Kind ActionKind

	// move
	DX, DY int

	// attack
	TargetID int
}
