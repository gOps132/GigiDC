package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"time"
)

type Handler func(ctx context.Context, payload json.RawMessage) error

type WorkerOptions struct {
	MaxWorkers   int
	PollInterval time.Duration
	WorkerID     string
}

type WorkerPool struct {
	queue        Queue
	logger       *slog.Logger
	handlers     map[string]Handler
	maxWorkers   int
	pollInterval time.Duration
	workerID     string
	mu           sync.Mutex
	active       int
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
}

func NewWorkerPool(queue Queue, logger *slog.Logger, opts WorkerOptions) *WorkerPool {
	maxWorkers := opts.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = 5
	}
	pollInterval := opts.PollInterval
	if pollInterval <= 0 {
		pollInterval = 5 * time.Second
	}
	workerID := opts.WorkerID
	if workerID == "" {
		workerID = "worker-default"
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &WorkerPool{
		queue:        queue,
		logger:       logger,
		handlers:     make(map[string]Handler),
		maxWorkers:   maxWorkers,
		pollInterval: pollInterval,
		workerID:     workerID,
	}
}

func (p *WorkerPool) Register(kind string, handler Handler) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers[kind] = handler
}

func (p *WorkerPool) Start(ctx context.Context) {
	p.mu.Lock()
	if p.cancel != nil {
		p.mu.Unlock()
		return
	}
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.mu.Unlock()

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.logger.Info("starting jobs worker pool", "worker_id", p.workerID, "max_workers", p.maxWorkers)
		ticker := time.NewTicker(p.pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-p.ctx.Done():
				p.logger.Info("stopping jobs worker pool poll loop")
				return
			case <-ticker.C:
				p.pollAndDispatch()
			}
		}
	}()
}

func (p *WorkerPool) Stop() {
	p.mu.Lock()
	if p.cancel == nil {
		p.mu.Unlock()
		return
	}
	p.cancel()
	p.mu.Unlock()

	p.wg.Wait()
	p.logger.Info("jobs worker pool stopped")
}

func (p *WorkerPool) pollAndDispatch() {
	p.mu.Lock()
	available := p.maxWorkers - p.active
	p.mu.Unlock()

	if available <= 0 {
		return
	}

	// Claim up to 'available' ready jobs
	jobs, err := p.queue.Claim(p.ctx, p.workerID, available)
	if err != nil {
		p.logger.Error("failed to claim jobs from queue", "error", err)
		return
	}

	for _, job := range jobs {
		p.mu.Lock()
		p.active++
		p.mu.Unlock()

		p.wg.Add(1)
		go func(j Job) {
			defer p.wg.Done()
			defer func() {
				p.mu.Lock()
				p.active--
				p.mu.Unlock()
			}()

			p.processJob(j)
		}(job)
	}
}

func (p *WorkerPool) processJob(job Job) {
	p.logger.Info("processing job", "job_id", job.ID, "kind", job.Kind, "attempt", job.Attempts)

	p.mu.Lock()
	handler, ok := p.handlers[job.Kind]
	p.mu.Unlock()

	if !ok {
		p.logger.Warn("no handler registered for job kind", "job_id", job.ID, "kind", job.Kind)
		err := p.queue.Fail(p.ctx, job.ID, errors.New("no handler registered"))
		if err != nil {
			p.logger.Error("failed to mark job as failed (missing handler)", "job_id", job.ID, "error", err)
		}
		return
	}

	// Create a execution context with a reasonable timeout (e.g., 5 minutes)
	jobCtx, cancel := context.WithTimeout(p.ctx, 5*time.Minute)
	defer cancel()

	err := handler(jobCtx, job.Payload)
	if err != nil {
		p.logger.Error("job execution failed", "job_id", job.ID, "kind", job.Kind, "error", err)
		if failErr := p.queue.Fail(p.ctx, job.ID, err); failErr != nil {
			p.logger.Error("failed to mark job as failed in queue", "job_id", job.ID, "error", failErr)
		}
		return
	}

	p.logger.Info("job completed successfully", "job_id", job.ID, "kind", job.Kind)
	if succErr := p.queue.Complete(p.ctx, job.ID); succErr != nil {
		p.logger.Error("failed to mark job as completed in queue", "job_id", job.ID, "error", succErr)
	}
}
