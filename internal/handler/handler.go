package handler

import (
	"github.com/zjunaidz/auditd/internal/queue"
	"github.com/zjunaidz/auditd/internal/service"
)

type Handler struct {
	svc *service.AuditService
	queue *queue.EventQueue
}

func New(svc *service.AuditService, queue *queue.EventQueue) *Handler {
	return &Handler{
		svc: svc,
		queue: queue,
	}
}
