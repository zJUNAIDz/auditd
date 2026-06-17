package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zjunaidz/auditd/internal/db"
	"github.com/zjunaidz/auditd/internal/model"
)

const genesisHash = "GENESIS_HASH"

type tenantEntry struct {
	tenant    db.GetTenantByAPIKeyRow
	expiresAt time.Time
}

type AuditService struct {
	pool        *pgxpool.Pool
	queries     *db.Queries
	hmacSecret  string
	cacheMU     sync.RWMutex
	tenantCache map[string]tenantEntry // apiKey -> tenant
}

func New(pool *pgxpool.Pool, hmacSecret string) *AuditService {
	return &AuditService{
		pool:        pool,
		queries:     db.New(pool),
		hmacSecret:  hmacSecret,
		tenantCache: make(map[string]tenantEntry),
	}
}

func (s *AuditService) ResolveTenant(ctx context.Context, apiKey string) (*db.GetTenantByAPIKeyRow, error) {
	s.cacheMU.RLock()
	entry, found := s.tenantCache[apiKey]
	s.cacheMU.RUnlock()

	if found && time.Until(entry.expiresAt) > 0 {
		return &entry.tenant, nil
	}
	tenant, err := s.queries.GetTenantByAPIKey(ctx, apiKey)
	if err != nil {
		return nil, err
	}
	s.cacheMU.Lock()
	s.tenantCache[apiKey] = tenantEntry{
		tenant:    tenant,
		expiresAt: time.Now().Add(5 * time.Minute), // Example expiration time
	}
	s.cacheMU.Unlock()
	return &tenant, nil
}

func (s *AuditService) IngestEvent(ctx context.Context, payload model.IngestPayload, tenantSecret string) (uuid.UUID, error) {

	var resultID uuid.UUID
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {

		qtx := s.queries.WithTx(tx)

		lockKey := tenantLockKey(payload.TenantID)
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey); err != nil {
			return err
		}

		prevHash := genesisHash
		lastHash, err := qtx.GetLastEventHash(ctx, pgtype.UUID{Bytes: payload.TenantID, Valid: true})
		if err == nil {
			prevHash = lastHash
		}

		hash := s.computeEventHash(payload, prevHash, tenantSecret)

		//Marshall metadata
		metaBytes, err := json.Marshal(payload.Input.Metadata)
		if err != nil {
			metaBytes = []byte("{}") // Fallback to empty JSON object
		}

		id, err := s.queries.InsertEvent(ctx, db.InsertEventParams{
			ID:           pgtype.UUID{Bytes: payload.ID, Valid: true},
			TenantID:     pgtype.UUID{Bytes: payload.TenantID, Valid: true},
			ActorID:      payload.Input.ActorID,
			ActorType:    payload.Input.ActorType,
			Action:       payload.Input.Action,
			ResourceID:   payload.Input.ResourceID,
			ResourceType: payload.Input.ResourceType,
			Metadata:     metaBytes,
			Timestamp:    pgtype.Timestamptz{Time: payload.Timestamp, Valid: true},
			CreatedAt:    pgtype.Timestamptz{Time: time.Now(), Valid: true},
			PrevHash:     prevHash,
			Hash:         hash,
		})
		if err != nil {
			return err
		}
		resultID = id.Bytes
		return nil
	})
	return resultID, err
}

func (s *AuditService) ListEvents(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]db.AuditEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 20 // Default limit
	}
	return s.queries.ListEvents(ctx, db.ListEventsParams{
		TenantID: pgtype.UUID{Bytes: tenantID, Valid: true},
		Limit:    int32(limit),
		Offset:   int32(offset),
	})
}

func (s *AuditService) computeEventHash(payload model.IngestPayload, prevHash string, tenantSecret string) string {
	secret := tenantSecret
	if secret == "" {
		secret = s.hmacSecret
	}
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		payload.ID.String(), payload.TenantID.String(), payload.Input.ActorID, payload.Input.Action, payload.Input.ResourceID, payload.Timestamp.UTC().Format(time.RFC3339Nano), prevHash)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

func tenantLockKey(id uuid.UUID) int64 {
	h := fnv.New64a()
	h.Write(id[:])
	return int64(h.Sum64())
}
