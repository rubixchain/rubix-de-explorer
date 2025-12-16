package services

import (
	"log"
	"runtime"
	"sync"
	"time"
)

type workerPool struct {
	name       string
	queue      chan func()
	minWorkers int
	maxWorkers int

	mu      sync.Mutex
	running int
}

type WorkerPoolStatus struct {
	Workers      int
	QueueLen     int
	QueueCap     int
	LoadFactor   float64
	BlockWorkers int
	TokenWorkers int
	SyncWorkers  int

	BlockQueueLen int
	TokenQueueLen int
	SyncQueueLen  int

	BlockQueueCap int
	TokenQueueCap int
	SyncQueueCap  int
}

var (
	blockPool *workerPool
	tokenPool *workerPool
	syncPool  *workerPool

	initPoolsOnce sync.Once
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func newWorkerPool(name string, queueSize, minWorkers, maxWorkers int) *workerPool {
	if minWorkers < 1 {
		minWorkers = 1
	}
	if maxWorkers < minWorkers {
		maxWorkers = minWorkers
	}

	p := &workerPool{
		name:       name,
		queue:      make(chan func(), queueSize),
		minWorkers: minWorkers,
		maxWorkers: maxWorkers,
	}

	for i := 0; i < minWorkers; i++ {
		p.startWorker()
	}
	log.Printf("🔧 %s pool started with %d workers (max %d)", name, minWorkers, maxWorkers)

	return p
}

func (p *workerPool) startWorker() {
	p.mu.Lock()
	p.running++
	id := p.running
	p.mu.Unlock()

	go func() {
		idleTimeout := 30 * time.Second
		for {
			select {
			case job, ok := <-p.queue:
				if !ok {
					return
				}
				if job != nil {
					job()
				}
			case <-time.After(idleTimeout):
				p.mu.Lock()
				if p.running > p.minWorkers {
					p.running--
					p.mu.Unlock()
					log.Printf("🧹 %s worker %d exiting due to idleness", p.name, id)
					return
				}
				p.mu.Unlock()
			}
		}
	}()
}

func (p *workerPool) maybeScaleUp() {
	p.mu.Lock()
	defer p.mu.Unlock()

	qLen := len(p.queue)
	if qLen > p.running && p.running < p.maxWorkers {
		// simple heuristic: if queue longer than workers, spawn one more
		p.startWorker()
		log.Printf("📈 %s pool scaled up: %d workers (queue=%d)", p.name, p.running, qLen)
	}
}

func (p *workerPool) enqueue(job func()) bool {
	select {
	case p.queue <- job:
		p.maybeScaleUp()
		return true
	default:
		// queue full → caller can decide to run inline
		return false
	}
}

// InitWorkerPools configures pools based on CPU cores
func InitWorkerPools(totalCPU int) {
	initPoolsOnce.Do(func() {
		if totalCPU <= 0 {
			totalCPU = runtime.NumCPU()
		}

		// Heuristics; you can tweak these
		maxBlock := max(2, totalCPU/2)
		maxToken := max(2, totalCPU/3)
		maxSync := max(2, totalCPU/2)

		blockPool = newWorkerPool("block", 2000, 2, maxBlock)
		tokenPool = newWorkerPool("token", 2000, 2, maxToken)
		syncPool = newWorkerPool("sync", 1000, 1, maxSync)
	})
}

// EnqueueBlockUpdateTask schedules a high-priority block job
func EnqueueBlockUpdateTask(job func()) bool {
	if blockPool == nil {
		InitWorkerPools(0)
	}
	return blockPool.enqueue(job)
}

// EnqueueTokenUpdateTask schedules a high-priority token job
func EnqueueTokenUpdateTask(table string, data interface{}, op string) bool {
	if tokenPool == nil {
		InitWorkerPools(0)
	}
	return tokenPool.enqueue(func() {
		UpdateTokens(table, data, op)
	})
}

// EnqueueBackgroundSyncTask schedules a background sync job
func EnqueueBackgroundSyncTask(job func()) bool {
	if syncPool == nil {
		InitWorkerPools(0)
	}
	return syncPool.enqueue(job)
}

// GetWorkerPoolStatus returns a snapshot for debugging/monitoring
func GetWorkerPoolStatus() WorkerPoolStatus {
	if blockPool == nil || tokenPool == nil || syncPool == nil {
		return WorkerPoolStatus{}
	}

	bLen := len(blockPool.queue)
	tLen := len(tokenPool.queue)
	sLen := len(syncPool.queue)

	bCap := cap(blockPool.queue)
	tCap := cap(tokenPool.queue)
	sCap := cap(syncPool.queue)

	totalLen := bLen + tLen + sLen
	totalCap := bCap + tCap + sCap

	load := 0.0
	if totalCap > 0 {
		load = float64(totalLen) / float64(totalCap)
	}

	blockPool.mu.Lock()
	bw := blockPool.running
	blockPool.mu.Unlock()

	tokenPool.mu.Lock()
	tw := tokenPool.running
	tokenPool.mu.Unlock()

	syncPool.mu.Lock()
	sw := syncPool.running
	syncPool.mu.Unlock()

	return WorkerPoolStatus{
		Workers:      bw + tw + sw,
		QueueLen:     totalLen,
		QueueCap:     totalCap,
		LoadFactor:   load,
		BlockWorkers: bw,
		TokenWorkers: tw,
		SyncWorkers:  sw,

		BlockQueueLen: bLen,
		TokenQueueLen: tLen,
		SyncQueueLen:  sLen,

		BlockQueueCap: bCap,
		TokenQueueCap: tCap,
		SyncQueueCap:  sCap,
	}
}
