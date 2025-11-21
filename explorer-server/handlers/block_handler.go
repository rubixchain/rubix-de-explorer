package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"explorer-server/services"
)

// ============ NOTIFICATION QUEUE (ADD THIS AT TOP OF FILE) ============

type NotificationQueue struct {
	queue          chan notificationTask
	workers        int
	wg             sync.WaitGroup
	isShuttingDown bool
	mu             sync.Mutex
}

type notificationTask struct {
	taskType   string      // "block" or "token"
	tableName  string      // for tokens
	data       interface{} // block map or token data
	operation  string      // CREATE, UPDATE, DELETE
	retryCount int
}

var globalQueue *NotificationQueue
var queueOnce sync.Once

func InitNotificationQueue(workers int) *NotificationQueue {
	queueOnce.Do(func() {
		globalQueue = &NotificationQueue{
			queue:   make(chan notificationTask, 5000),
			workers: workers,
		}
		globalQueue.startWorkers()
	})
	return globalQueue
}

func (nq *NotificationQueue) startWorkers() {
	for i := 0; i < nq.workers; i++ {
		nq.wg.Add(1)
		go nq.worker(i)
	}
	log.Printf("🔄 Started %d notification workers", nq.workers)
}

func (nq *NotificationQueue) worker(id int) {
	defer nq.wg.Done()

	for task := range nq.queue {
		start := time.Now()
		err := nq.processTask(task)
		duration := time.Since(start)

		if err != nil {
			log.Printf("⚠️ Worker %d: failed to process %s (retry %d, took %v): %v",
				id, task.taskType, task.retryCount, duration.Round(time.Millisecond), err)

			if task.retryCount < 3 {
				task.retryCount++
				backoff := time.Duration(1<<uint(task.retryCount-1)) * 500 * time.Millisecond
				go func(t notificationTask) {
					time.Sleep(backoff)
					select {
					case nq.queue <- t:
					default:
						log.Printf("❌ Failed to requeue task")
					}
				}(task)
			}
		} else {
			log.Printf("✅ Worker %d: processed %s in %v", id, task.taskType, duration.Round(time.Millisecond))
		}
	}
}

func (nq *NotificationQueue) processTask(task notificationTask) error {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ Panic in task processing: %v", r)
		}
	}()

	if task.taskType == "block" {
		blockMap, ok := task.data.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid block data type")
		}
		services.UpdateBlocks(blockMap)
		return nil
	} else if task.taskType == "token" {
		services.UpdateTokens(task.tableName, task.data, task.operation)
		return nil
	}
	return fmt.Errorf("unknown task type: %s", task.taskType)
}

func (nq *NotificationQueue) EnqueueBlockUpdate(blockMap map[string]interface{}) error {
	nq.mu.Lock()
	if nq.isShuttingDown {
		nq.mu.Unlock()
		return fmt.Errorf("notification queue is shutting down")
	}
	nq.mu.Unlock()

	task := notificationTask{
		taskType: "block",
		data:     blockMap,
	}

	select {
	case nq.queue <- task:
		return nil
	case <-time.After(3 * time.Second):
		return fmt.Errorf("queue timeout")
	}
}

func (nq *NotificationQueue) EnqueueTokenUpdate(tableName string, data interface{}, operation string) error {
	nq.mu.Lock()
	if nq.isShuttingDown {
		nq.mu.Unlock()
		return fmt.Errorf("notification queue is shutting down")
	}
	nq.mu.Unlock()

	task := notificationTask{
		taskType:  "token",
		tableName: tableName,
		data:      data,
		operation: operation,
	}

	select {
	case nq.queue <- task:
		return nil
	case <-time.After(3 * time.Second):
		return fmt.Errorf("queue timeout")
	}
}

func (nq *NotificationQueue) Shutdown(ctx context.Context) error {
	nq.mu.Lock()
	nq.isShuttingDown = true
	nq.mu.Unlock()

	close(nq.queue)

	done := make(chan struct{})
	go func() {
		nq.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("✅ Notification queue shut down successfully")
		return nil
	case <-ctx.Done():
		log.Printf("⚠️ Notification queue shutdown timeout")
		return fmt.Errorf("shutdown timeout")
	}
}

func GetQueue() *NotificationQueue {
	if globalQueue == nil {
		return InitNotificationQueue(8)
	}
	return globalQueue
}

// ============ EXISTING HANDLERS (KEEP AS-IS) ============

func GetTxnsCountHandler(w http.ResponseWriter, r *http.Request) {
	count, err := services.GetTxnsCount()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]int64{"all_block_count": count}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetTransferBlockListHandler(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	pageStr := r.URL.Query().Get("page")
	limit := 10
	page := 1

	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}

	response, err := services.GetTransferBlocksList(limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetBlockInfoFromTxnHash(w http.ResponseWriter, r *http.Request) {
	txnHash := r.URL.Query().Get("hash")

	response, err := services.GetTransferBlockInfoFromTxnID(txnHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetBlockInfoFromBlockHash(w http.ResponseWriter, r *http.Request) {
	blockHash := r.URL.Query().Get("hash")

	response, err := services.GetTransferBlockInfoFromBlockHash(blockHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetBurntTxnInfoFromTxnHash(w http.ResponseWriter, r *http.Request) {
	txnkHash := r.URL.Query().Get("hash")

	data, err := services.GetBurntBlockInfo(txnkHash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func GetBurntBlockList(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	pageStr := r.URL.Query().Get("page")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}

	page, err := strconv.Atoi(pageStr)
	if err != nil || page <= 0 {
		page = 1
	}

	data, err := services.GetBurntBlockList(limit, page)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// ============ UPDATED HANDLERS - NOW ASYNC ============

// UpdateBlocksHandler - ASYNC (queues instead of blocking)
func UpdateBlocksHandler(w http.ResponseWriter, r *http.Request) {
	var block map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&block); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Println("📝 Received block from fullnode, queueing for processing")

	queue := GetQueue()
	if err := queue.EnqueueBlockUpdate(block); err != nil {
		http.Error(w, fmt.Sprintf("Queue error: %v", err), http.StatusServiceUnavailable)
		log.Printf("❌ Failed to enqueue block: %v", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "queued", "message": "Block update accepted for processing"}`))
}

// QueueStatusHandler - Check queue status
func QueueStatusHandler(w http.ResponseWriter, r *http.Request) {
	queue := GetQueue()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "active",
		"queue_length":     len(queue.queue),
		"max_capacity":     5000,
		"workers":          queue.workers,
		"is_shutting_down": queue.isShuttingDown,
		"timestamp":        time.Now().Format(time.RFC3339),
	})
}
