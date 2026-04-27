package processor

import (
	"context"
	"explorer-server/model"
	"explorer-server/util"
	"log"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// GlobalWorkerPool is the singleton instance of the dynamic worker pool
var GlobalWorkerPool *DynamicWorkerPool

// DynamicWorkerPool handles adaptive concurrent transaction processing for the Explorer
type DynamicWorkerPool struct {
	txnQueue      chan *model.EventTransaction
	processedTxns sync.Map
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup

	// Dynamic scaling configuration
	minWorkers     int
	maxWorkers     int
	currentWorkers int
	workersMutex   sync.RWMutex

	// System monitoring
	memoryThreshold float64
	queueThreshold  int
	scaleUpDelay    time.Duration
	scaleDownDelay  time.Duration
	lastScaleAction time.Time

	// Metrics
	queueLength        int64
	averageProcessTime time.Duration
	receivedTxnCount   int64
	processedTxnCount  int64
	metricsMu          sync.Mutex

	// Worker management
	workerChannels  map[int]chan struct{}
	workerChanMutex sync.RWMutex
}

// InitDynamicWorkerPool initializes the global dynamic worker pool
func InitDynamicWorkerPool() {
	if GlobalWorkerPool != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	numCPU := runtime.NumCPU()

	// Determine max workers. Because workload is heavily DB I/O bound,
	// we can safely exceed physical CPU cores. Cap at 80 (since MaxOpenConns is 100).
	maxW := numCPU * 4
	if maxW > 80 {
		maxW = 80
	}
	if maxW < 1 {
		maxW = 1
	}

	GlobalWorkerPool = &DynamicWorkerPool{
		txnQueue:        make(chan *model.EventTransaction, 20000),
		ctx:             ctx,
		cancel:          cancel,
		minWorkers:      mathMax(1, numCPU/4),
		maxWorkers:      maxW,
		currentWorkers:  mathMax(1, numCPU/2),
		memoryThreshold: 75.0,
		queueThreshold:  100,
		scaleUpDelay:    time.Second * 10,
		scaleDownDelay:  time.Second * 30,
		workerChannels:  make(map[int]chan struct{}),
	}

	log.Printf("Initializing Dynamic Worker Pool: Min=%d, Max=%d, Current=%d workers\n",
		GlobalWorkerPool.minWorkers, GlobalWorkerPool.maxWorkers, GlobalWorkerPool.currentWorkers)

	// Start initial workers
	for i := 0; i < GlobalWorkerPool.currentWorkers; i++ {
		GlobalWorkerPool.startWorker(i)
	}

	// Start system monitor
	go GlobalWorkerPool.systemMonitor()
}

// EnqueueTransaction adds a transaction to the processing queue
func (p *DynamicWorkerPool) EnqueueTransaction(txnEvent *model.EventTransaction) {
	atomic.AddInt64(&p.receivedTxnCount, 1)
	atomic.StoreInt64(&p.queueLength, int64(len(p.txnQueue)))

	select {
	case p.txnQueue <- txnEvent:
		// Successfully queued

	case <-time.After(20 * time.Second):
		log.Printf("Warning: Failed to queue transaction %s - queue full (length=%d)\n",
			txnEvent.Transaction.ID, len(p.txnQueue))
		return

	case <-p.ctx.Done():
		log.Println("Worker pool is shutting down, dropping transaction")
		return
	}
}

// Monitor system resources and adjust worker count
func (p *DynamicWorkerPool) systemMonitor() {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.evaluateAndScale()
		case <-p.ctx.Done():
			return
		}
	}
}

// Evaluate system conditions and decide on scaling
func (p *DynamicWorkerPool) evaluateAndScale() {
	memoryUsagePercent := util.GetSystemMemoryUsage()
	queueLen := int64(len(p.txnQueue))

	p.workersMutex.RLock()
	currentWorkers := p.currentWorkers
	p.workersMutex.RUnlock()

	scalingDecision := p.determineScalingAction(memoryUsagePercent, queueLen, currentWorkers)

	switch scalingDecision {
	case "scale_up":
		p.scaleUp()
	case "scale_down":
		p.scaleDown()
	}
}

func (p *DynamicWorkerPool) determineScalingAction(memoryPercent float64, queueLen int64, currentWorkers int) string {
	now := time.Now()

	queuePressure := queueLen > int64(p.queueThreshold)
	resourcesAvailable := memoryPercent < p.memoryThreshold
	hasWorkload := queueLen > 10
	canScaleUp := currentWorkers < p.maxWorkers
	scaleUpDelayMet := now.Sub(p.lastScaleAction) > p.scaleUpDelay

	shouldScaleUp := (queuePressure || (hasWorkload && resourcesAvailable)) &&
		canScaleUp &&
		scaleUpDelayMet

	highResourceUsage := memoryPercent > p.memoryThreshold
	lowWorkload := queueLen == 0 && currentWorkers > p.minWorkers
	canScaleDown := currentWorkers > p.minWorkers
	scaleDownDelayMet := now.Sub(p.lastScaleAction) > p.scaleDownDelay

	shouldScaleDown := (highResourceUsage || lowWorkload) &&
		canScaleDown &&
		scaleDownDelayMet

	if shouldScaleUp {
		return "scale_up"
	} else if shouldScaleDown {
		return "scale_down"
	}

	return "no_change"
}

func (p *DynamicWorkerPool) scaleUp() {
	p.workersMutex.Lock()
	defer p.workersMutex.Unlock()

	if p.currentWorkers >= p.maxWorkers {
		return
	}

	newWorkers := mathMax(1, p.currentWorkers/4)
	newWorkers = mathMin(newWorkers, p.maxWorkers-p.currentWorkers)

	for i := 0; i < newWorkers; i++ {
		workerID := p.currentWorkers + i
		p.startWorker(workerID)
	}

	p.currentWorkers += newWorkers
	p.lastScaleAction = time.Now()
}

func (p *DynamicWorkerPool) scaleDown() {
	p.workersMutex.Lock()
	defer p.workersMutex.Unlock()

	if p.currentWorkers <= p.minWorkers {
		return
	}

	removeWorkers := mathMax(1, p.currentWorkers/4)
	removeWorkers = mathMin(removeWorkers, p.currentWorkers-p.minWorkers)

	p.workerChanMutex.Lock()
	// Robust scaling: collect and sort live IDs to ensure we stop the newest ones first
	liveIDs := make([]int, 0, len(p.workerChannels))
	for id := range p.workerChannels {
		liveIDs = append(liveIDs, id)
	}
	sort.Ints(liveIDs)

	stopped := 0
	for i := len(liveIDs) - 1; i >= 0 && stopped < removeWorkers; i-- {
		id := liveIDs[i]
		if id < p.minWorkers {
			break
		}
		if stopChan, exists := p.workerChannels[id]; exists {
			close(stopChan)
			delete(p.workerChannels, id)
			stopped++
		}
	}
	p.workerChanMutex.Unlock()

	p.currentWorkers -= stopped
	p.lastScaleAction = time.Now()
}

func (p *DynamicWorkerPool) startWorker(workerID int) {
	stopChan := make(chan struct{})

	p.workerChanMutex.Lock()
	p.workerChannels[workerID] = stopChan
	p.workerChanMutex.Unlock()

	p.wg.Add(1)
	go p.dynamicWorker(workerID, stopChan)
}

func (p *DynamicWorkerPool) dynamicWorker(workerID int, stopChan chan struct{}) {
	defer p.wg.Done()

	for {
		select {
		case txnEvent := <-p.txnQueue:
			// Safety: If RAM is critical, don't start new work to prevent OOM
			for util.GetSystemMemoryUsage() > 85 {
				log.Printf("[Worker %d] CRITICAL RAM (>85%%). Pausing ingestion to prevent crash...", workerID)
				time.Sleep(2 * time.Second)
			}

			// Layer 3: Recovery protection for the top-level worker thread
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[Worker %d] CRITICAL RECOVERY: Rescued from panic during transaction %s: %v", workerID, txnEvent.TransactionID, r)
					}
				}()
				startTime := time.Now()

				// Handle the actual DB processing
				ProcessDBTransaction(txnEvent, workerID)

				atomic.AddInt64(&p.processedTxnCount, 1)
				processingTime := time.Since(startTime)
				p.updateProcessingMetrics(processingTime)
			}()

		case <-stopChan:
			return

		case <-p.ctx.Done():
			return
		}
	}
}

func (p *DynamicWorkerPool) updateProcessingMetrics(processingTime time.Duration) {
	p.metricsMu.Lock()
	defer p.metricsMu.Unlock()
	if p.averageProcessTime == 0 {
		p.averageProcessTime = processingTime
	} else {
		alpha := 0.1
		p.averageProcessTime = time.Duration(
			float64(p.averageProcessTime)*(1-alpha) +
				float64(processingTime)*alpha,
		)
	}
}

// Shutdown gracefully stops all workers
func (p *DynamicWorkerPool) Shutdown() {
	log.Println("Shutting down dynamic worker pool...")
	p.cancel()
	close(p.txnQueue)

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All workers shut down gracefully")
	case <-time.After(30 * time.Second):
		log.Println("Warning: Workers shutdown timeout - forcing termination")
	}
}

// mathMin helper
func mathMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// mathMax helper
func mathMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// GetStats returns current status of the dynamic worker pool
func (p *DynamicWorkerPool) GetStats() map[string]interface{} {
	p.workersMutex.RLock()
	currentWorkers := p.currentWorkers
	p.workersMutex.RUnlock()

	return map[string]interface{}{
		"workers":           currentWorkers,
		"queue_length":      len(p.txnQueue),
		"queue_cap":         cap(p.txnQueue),
		"received_txns":     atomic.LoadInt64(&p.receivedTxnCount),
		"processed_txns":    atomic.LoadInt64(&p.processedTxnCount),
		"avg_process_time":  p.averageProcessTime.String(),
	}
}
