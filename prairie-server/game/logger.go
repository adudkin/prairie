package game

type Logger struct {
	buf []LogEntry
	cap int
}

func NewLogger(capacity int) *Logger {
	return &Logger{buf: make([]LogEntry, 0, capacity), cap: capacity}
}

func (l *Logger) Add(le LogEntry) {
	if len(l.buf) >= l.cap {
		copy(l.buf[0:], l.buf[1:])
		l.buf = l.buf[:l.cap-1]
	}
	l.buf = append(l.buf, le)
}

func (l *Logger) Snapshot() []LogEntry {
	out := make([]LogEntry, len(l.buf))
	copy(out, l.buf)
	return out
}
