package handler

import "github.com/zjunaidz/auditd/internal/service"

type Handler struct {
	svc *service.AuditService
}

func New(svc *service.AuditService) *Handler {
	return &Handler{
		svc: svc,
	}
}
