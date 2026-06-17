package worker

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/zjunaidz/auditd/internal/model"
	"github.com/zjunaidz/auditd/internal/service"
)

type Pool struct {
	pool    <-chan model.IngestPayload
	svc     *service.AuditService
	workers int
}

func New(q <-chan model.IngestPayload, svc *service.AuditService, n int) *Pool {
	return &Pool{
		pool:    q,
		svc:     svc,
		workers: n,
	}
}

func (p *Pool) Start(ctx context.Context, wg *sync.WaitGroup) {
	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go p.runWorker(ctx, wg, i)
	}
}

func (p *Pool) runWorker(ctx context.Context, wg *sync.WaitGroup, id int) {
	defer wg.Done()

	for {
		select {
		case payload, ok := <-p.pool:
			if !ok {
				return // channel closed, exit worker
			}
			p.processWithRetry(ctx, payload)

		case <-ctx.Done():
			p.drainRemaining()
			return
		}
	}
}

func (p *Pool) drainRemaining() {
	for {
		select {
		case payload, ok := <-p.pool:
			if !ok {
				return // channel closed, exit
			}
			p.processWithRetry(context.Background(), payload)
		default:
			return // no more items, exit
		}
	}
}

func (p *Pool) processWithRetry(ctx context.Context, payload model.IngestPayload) {
	const maxRetries = 5

	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := p.svc.IngestEvent(ctx, payload, payload.TenantSecret)
		if err == nil {
			return
		}

		if attempt == maxRetries {
			// Log the failure after max retries
			log.Printf("Failed to process payload after %d attempts: %v", maxRetries, err)
			return
		}

		backoffDuration := time.Duration(attempt*attempt) * 100 * time.Millisecond // Exponential backoff
		time.Sleep(backoffDuration)
	}
}
