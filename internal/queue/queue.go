package queue

import "github.com/zjunaidz/auditd/internal/model"

type EventQueue struct {
	ch chan model.IngestPayload
}

func New(size int) *EventQueue {
	return &EventQueue{
		ch: make(chan model.IngestPayload, size),
	}
}

func (q *EventQueue) Enqueue(p model.IngestPayload) bool {
	select {
	case q.ch <- p:
		return true
	default:
		return false
	}
}

func (q *EventQueue) Chan() <-chan model.IngestPayload {
	return q.ch
}

func (q *EventQueue) Close() {
	close(q.ch)
}
