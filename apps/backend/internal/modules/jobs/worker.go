package jobs

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cubycloud/backend/internal/database"
	"github.com/cubycloud/backend/internal/sse"
	"github.com/uptrace/bun"
)

type HandlerFunc func(ctx context.Context, job *database.Job, updateProgress func(int)) (any, error)

type WorkerPool struct {
	db          *bun.DB
	queue       *Queue
	hub         *sse.Hub
	handlers    map[string]HandlerFunc
	handlersMu  sync.RWMutex
	concurrency int
	wakeChan    chan struct{}
	stopChan    chan struct{}
	wg          sync.WaitGroup
	running     atomic.Bool
}

func NewWorkerPool(db *bun.DB, queue *Queue, hub *sse.Hub) *WorkerPool {
	concurrency := 2
	var val string
	if err := db.QueryRow(`SELECT value FROM settings WHERE key = 'worker_concurrency_limit'`).Scan(&val); err == nil {
		if c, err := strconv.Atoi(val); err == nil && c > 0 {
			concurrency = c
		}
	}

	return &WorkerPool{
		db:          db,
		queue:       queue,
		hub:         hub,
		handlers:    make(map[string]HandlerFunc),
		concurrency: concurrency,
		wakeChan:    make(chan struct{}, 64),
		stopChan:    make(chan struct{}),
	}
}

func (p *WorkerPool) RegisterHandler(jobType string, fn HandlerFunc) {
	p.handlersMu.Lock()
	defer p.handlersMu.Unlock()
	p.handlers[jobType] = fn
}

func (p *WorkerPool) Start(ctx context.Context) {
	if p.running.Swap(true) {
		return
	}

	// Recover interrupted jobs on startup
	if recovered, err := p.queue.RecoverInterrupted(ctx); err == nil && recovered > 0 {
		log.Printf("[WorkerPool] Recovered %d interrupted jobs to FAILED state", recovered)
	}

	for i := range p.concurrency {
		p.wg.Add(1)
		go p.workerLoop(ctx, i)
	}
}

func (p *WorkerPool) Stop() {
	if !p.running.Swap(false) {
		return
	}
	close(p.stopChan)
	p.wg.Wait()
}

func (p *WorkerPool) notifySSE(jobID, jobType, userID, status string, progress int, result any, errMsg *string) {
	if p.hub == nil {
		return
	}
	p.hub.Broadcast(sse.Event{
		Type:   "job_updated",
		UserID: userID,
		Data: map[string]any{
			"id":            jobID,
			"type":          jobType,
			"status":        status,
			"progress":      progress,
			"result":        result,
			"error_message": errMsg,
		},
	})
}

func (p *WorkerPool) workerLoop(ctx context.Context, workerID int) {
	defer p.wg.Done()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ctx.Done():
			return
		default:
		}

		job, err := p.queue.Claim(ctx)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if job == nil {
			// No pending job, wait for wake signal, ticker, or shutdown
			select {
			case <-p.stopChan:
				return
			case <-ctx.Done():
				return
			case <-p.queue.WakeChan():
			case <-ticker.C:
			}
			continue
		}

		// Job claimed: dispatch to handler
		p.handlersMu.RLock()
		handler, ok := p.handlers[job.Type]
		p.handlersMu.RUnlock()

		if !ok {
			errMsg := fmt.Sprintf("no handler registered for job type: %s", job.Type)
			_ = p.queue.Fail(ctx, job.ID, errMsg)
			p.notifySSE(job.ID, job.Type, job.UserID, StatusFailed, 0, nil, &errMsg)
			continue
		}

		p.notifySSE(job.ID, job.Type, job.UserID, StatusProcessing, 0, nil, nil)

		updateProgress := func(prog int) {
			_ = p.queue.UpdateProgress(ctx, job.ID, prog)
			p.notifySSE(job.ID, job.Type, job.UserID, StatusProcessing, prog, nil, nil)
		}

		result, err := handler(ctx, job, updateProgress)
		if err != nil {
			errMsg := err.Error()
			_ = p.queue.Fail(ctx, job.ID, errMsg)
			p.notifySSE(job.ID, job.Type, job.UserID, StatusFailed, 0, nil, &errMsg)
		} else {
			_ = p.queue.Complete(ctx, job.ID, result)
			p.notifySSE(job.ID, job.Type, job.UserID, StatusCompleted, 100, result, nil)
		}
	}
}
