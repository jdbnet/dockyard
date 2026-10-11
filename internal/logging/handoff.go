package logging

// handoff passes finished batches to the spool writer without ever blocking the caller.
type handoff struct {
	ch chan Record
}

func newHandoff() *handoff {
	return &handoff{ch: make(chan Record, 1)}
}

// Submit queues a record. It returns false immediately when the writer is busy.
func (h *handoff) Submit(rec Record) bool {
	select {
	case h.ch <- rec:
		return true
	default:
		return false
	}
}
