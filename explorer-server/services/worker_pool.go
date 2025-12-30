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
	Workers    int     `json:"workers"`
	QueueLen   int     `json:"queue_len"`
	QueueCap   int     `json:"queue_cap"`
	LoadFactor float64 `json:"load_factor"`
}

var (
	blockPool     *workerPool
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

// InitWorkerPools configures the single block pool based on CPU cores
func InitWorkerPools(totalCPU int) {
	initPoolsOnce.Do(func() {
		if totalCPU <= 0 {
			totalCPU = runtime.NumCPU()
		}

		// Simple heuristic: half the cores for block workers, min 2
		maxBlock := max(2, totalCPU/2)

		blockPool = newWorkerPool("block", 2000, 2, maxBlock)
	})
}

// EnqueueBlockUpdateTask schedules a high-priority block job
func EnqueueBlockUpdateTask(job func()) bool {
	if blockPool == nil {
		InitWorkerPools(0)
	}
	return blockPool.enqueue(job)
}

// GetWorkerPoolStatus returns a snapshot for debugging/monitoring
func GetWorkerPoolStatus() WorkerPoolStatus {
	if blockPool == nil {
		return WorkerPoolStatus{}
	}

	bLen := len(blockPool.queue)
	bCap := cap(blockPool.queue)

	load := 0.0
	if bCap > 0 {
		load = float64(bLen) / float64(bCap)
	}

	blockPool.mu.Lock()
	bw := blockPool.running
	blockPool.mu.Unlock()

	return WorkerPoolStatus{
		Workers:    bw,
		QueueLen:   bLen,
		QueueCap:   bCap,
		LoadFactor: load,
	}
}
