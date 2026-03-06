package services

import (
	"context"
	"explorer-server/model"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// GlobalTxnProcessor is the singleton instance of the dynamic transaction processor
var GlobalTxnProcessor *DynamicTxnProcessor

// DynamicTxnProcessor handles adaptive concurrent transaction processing for the Explorer
type DynamicTxnProcessor struct {
	txnQueue      chan *model.PubSubTxnInfo
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
	processedTxnCount  int64

	// Worker management
	workerChannels  map[int]chan struct{}
	workerChanMutex sync.RWMutex

	// Resource monitor
	resourceMonitor *ResourceMonitor
}

// InitDynamicTxnProcessor initializes the global dynamic transaction processor
func InitDynamicTxnProcessor() {
	if GlobalTxnProcessor != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	numCPU := runtime.NumCPU()

	// Determine max workers. We reserve at least 1 core for the HTTP server
	// and DB connections to ensure the UI remains seamless under heavy load.
	maxW := numCPU - 1
	if maxW < 1 {
		maxW = 1
	}

	GlobalTxnProcessor = &DynamicTxnProcessor{
		txnQueue:        make(chan *model.PubSubTxnInfo, 2000), // Slightly larger buffer for Explorer
		ctx:             ctx,
		cancel:          cancel,
		minWorkers:      mathMax(1, numCPU/4),
		maxWorkers:      maxW,
		currentWorkers:  mathMax(1, numCPU/2),
		memoryThreshold: 75.0, // Scale down/avoid scale up if memory usage > 75%
		queueThreshold:  100,
		scaleUpDelay:    time.Second * 10,
		scaleDownDelay:  time.Second * 30,
		workerChannels:  make(map[int]chan struct{}),
		resourceMonitor: &ResourceMonitor{},
	}

	log.Printf("⚙️ Initializing Dynamic Transaction Processor: Min=%d, Max=%d, Current=%d workers\n",
		GlobalTxnProcessor.minWorkers, GlobalTxnProcessor.maxWorkers, GlobalTxnProcessor.currentWorkers)

	// Start initial workers
	for i := 0; i < GlobalTxnProcessor.currentWorkers; i++ {
		GlobalTxnProcessor.startWorker(i)
	}

	// Start system monitor
	go GlobalTxnProcessor.systemMonitor()
}

// EnqueueTransaction adds a transaction to the processing queue with timeout handling
func (p *DynamicTxnProcessor) EnqueueTransaction(txnEvent *model.PubSubTxnInfo) {
	currentQueueLen := int64(len(p.txnQueue))
	p.queueLength = currentQueueLen

	select {
	case p.txnQueue <- txnEvent:
		// Successfully queued

	case <-time.After(5 * time.Second):
		log.Printf("⚠️ Failed to queue transaction %s - queue full (length=%d)\n",
			txnEvent.BlockHash, len(p.txnQueue))

		if currentQueueLen > int64(p.queueThreshold) {
			log.Printf("⚠️ Queue threshold exceeded - dynamic scaling active (current=%d, threshold=%d)\n",
				currentQueueLen, p.queueThreshold)
		}
		return

	case <-p.ctx.Done():
		log.Println("ℹ️ Transaction processor is shutting down, dropping transaction")
		return
	}
}

// Monitor system resources and adjust worker count
func (p *DynamicTxnProcessor) systemMonitor() {
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
func (p *DynamicTxnProcessor) evaluateAndScale() {
	// Get memory stats
	resourceStats := p.resourceMonitor.GetResourceStats()
	memoryUsagePercent := resourceStats["memory_usage_pct"].(float64)

	// Get queue metrics
	queueLen := int64(len(p.txnQueue))

	p.workersMutex.RLock()
	currentWorkers := p.currentWorkers
	p.workersMutex.RUnlock()

	// Determine scaling action
	// (Excluding CPU percent for cross-platform simplicity, relying on memory and queue)
	scalingDecision := p.determineScalingAction(memoryUsagePercent, queueLen, currentWorkers)

	// Apply scaling decision
	switch scalingDecision {
	case "scale_up":
		p.scaleUp()
	case "scale_down":
		p.scaleDown()
	}
}

// Determine scaling action based on metrics
func (p *DynamicTxnProcessor) determineScalingAction(memoryPercent float64, queueLen int64, currentWorkers int) string {
	now := time.Now()

	// Scale up conditions
	queuePressure := queueLen > int64(p.queueThreshold)
	resourcesAvailable := memoryPercent < p.memoryThreshold
	hasWorkload := queueLen > 10
	canScaleUp := currentWorkers < p.maxWorkers
	scaleUpDelayMet := now.Sub(p.lastScaleAction) > p.scaleUpDelay

	shouldScaleUp := (queuePressure || (hasWorkload && resourcesAvailable)) &&
		canScaleUp &&
		scaleUpDelayMet

	// Scale down conditions
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

// Scale up worker count
func (p *DynamicTxnProcessor) scaleUp() {
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
	log.Printf("📈 Scaled UP transaction workers to %d\n", p.currentWorkers)
}

// Scale down worker count
func (p *DynamicTxnProcessor) scaleDown() {
	p.workersMutex.Lock()
	defer p.workersMutex.Unlock()

	if p.currentWorkers <= p.minWorkers {
		return
	}

	removeWorkers := mathMax(1, p.currentWorkers/4)
	removeWorkers = mathMin(removeWorkers, p.currentWorkers-p.minWorkers)

	p.workerChanMutex.Lock()
	workersToStop := make([]int, 0, removeWorkers)

	for workerID := p.currentWorkers - 1; len(workersToStop) < removeWorkers && workerID >= p.minWorkers; workerID-- {
		if stopChan, exists := p.workerChannels[workerID]; exists {
			close(stopChan)
			delete(p.workerChannels, workerID)
			workersToStop = append(workersToStop, workerID)
		}
	}
	p.workerChanMutex.Unlock()

	p.currentWorkers -= len(workersToStop)
	p.lastScaleAction = time.Now()

	if len(workersToStop) > 0 {
		log.Printf("📉 Scaled DOWN transaction workers to %d\n", p.currentWorkers)
	}
}

// Start a new worker
func (p *DynamicTxnProcessor) startWorker(workerID int) {
	stopChan := make(chan struct{})

	p.workerChanMutex.Lock()
	p.workerChannels[workerID] = stopChan
	p.workerChanMutex.Unlock()

	p.wg.Add(1)
	go p.dynamicWorker(workerID, stopChan)
}

// Dynamic worker that can be individually stopped
func (p *DynamicTxnProcessor) dynamicWorker(workerID int, stopChan chan struct{}) {
	defer p.wg.Done()

	for {
		select {
		case txnEvent := <-p.txnQueue:
			startTime := time.Now()

			// Process the transaction
			ProcessSingleTransaction(txnEvent, workerID)

			// Incremement count and tracking
			atomic.AddInt64(&p.processedTxnCount, 1)
			processingTime := time.Since(startTime)
			p.updateProcessingMetrics(processingTime)

		case <-stopChan:
			return

		case <-p.ctx.Done():
			return
		}
	}
}

// Update processing metrics for better scaling decisions
func (p *DynamicTxnProcessor) updateProcessingMetrics(processingTime time.Duration) {
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
func (p *DynamicTxnProcessor) Shutdown() {
	log.Println("🛑 Shutting down dynamic transaction processor...")
	p.cancel()
	close(p.txnQueue)

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("✅ All transaction workers shut down gracefully")
	case <-time.After(30 * time.Second):
		log.Println("⚠️ Transaction workers shutdown timeout - forcing termination")
	}
}

// ProcessSingleTransaction handles the actual logic of logging/inserting to DB
func ProcessSingleTransaction(newEvent *model.PubSubTxnInfo, workerID int) {
	// For now, mimic the existing TxnCallBack logging
	log.Printf("[Worker %d] 📥 Processed transaction from PubSub:", workerID)
	log.Printf("   BlockHash:    %s", newEvent.BlockHash)
	log.Printf("   BlockType:    %s", newEvent.BlockType)
	log.Printf("   AssetType:    %d", newEvent.AssetType)
	log.Printf("   PublisherDID: %s", newEvent.PublisherDID)
	log.Printf("   ReceiverDID:  %s", newEvent.ReceiverDID)
	log.Printf("   TxnValue:     %f", newEvent.TransactionValue)
	log.Printf("   TokenValue:   %f", newEvent.TokenValue)
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

// GetStats returns current status of the dynamic processor for monitoring endpoints
func (p *DynamicTxnProcessor) GetStats() map[string]interface{} {
	p.workersMutex.RLock()
	currentWorkers := p.currentWorkers
	p.workersMutex.RUnlock()

	return map[string]interface{}{
		"workers":      currentWorkers,
		"queue_length": len(p.txnQueue),
		"queue_cap":    cap(p.txnQueue),
	}
}
