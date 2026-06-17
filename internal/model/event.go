package model

import (
	"time"

	"github.com/google/uuid"
)

type EventInput struct {
	ActorID      string                 `json:"actor_id"      binding:"required"`
	ActorType    string                 `json:"actor_type"    binding:"required,oneof=user service system"`
	Action       string                 `json:"action"        binding:"required"`
	ResourceType string                 `json:"resource_type" binding:"required"`
	ResourceID   string                 `json:"resource_id"   binding:"required"`
	Metadata     map[string]interface{} `json:"metadata"`
}

type IngestPayload struct {
	Input     EventInput
	TenantID  uuid.UUID
	Timestamp time.Time
	ID        uuid.UUID
}