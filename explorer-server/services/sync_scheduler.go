package services

import (
	"log"
	"time"
)

// StartContinuousSync starts a continuous background sync loop that:
// 1. Runs an initial full sync (asset lists -> token chains)
// 2. Then periodically refreshes asset lists and token chains
//
// It uses the dedicated sync worker pool via EnqueueBackgroundSyncTask,
// so it will NOT block/slow block or token live updates.
func StartContinuousSync(maxParallel int) {
	go func() {
		log.Printf("🔁 Continuous background sync started (sync workers up to %d)\n", maxParallel)

		// -----------------------------
		// Initial full sync (once)
		// -----------------------------
		ok := EnqueueBackgroundSyncTask(func() {
			start := time.Now()
			log.Println("🔄 [SYNC] Initial full asset + token-chain sync STARTED")

			// 1) Asset lists (RBT / FT / NFT / SC)
			if err := FetchAndStoreAllRBTsFromFullNodeDB(); err != nil {
				log.Printf("⚠️ [SYNC] FetchAndStoreAllRBTsFromFullNodeDB error: %v", err)
			}
			if err := FetchAndStoreAllFTsFromFullNodeDB(); err != nil {
				log.Printf("⚠️ [SYNC] FetchAndStoreAllFTsFromFullNodeDB error: %v", err)
			}
			if err := FetchAndStoreAllNFTsFromFullNodeDB(); err != nil {
				log.Printf("⚠️ [SYNC] FetchAndStoreAllNFTsFromFullNodeDB error: %v", err)
			}
			if err := FetchAndStoreAllSCsFromFullNodeDB(); err != nil {
				log.Printf("⚠️ [SYNC] FetchAndStoreAllSCsFromFullNodeDB error: %v", err)
			}

			// 2) Heavy get-token-chain sync
			if err := FetchAllTokenChainFromFullNode(); err != nil {
				log.Printf("⚠️ [SYNC] FetchAllTokenChainFromFullNode error: %v", err)
			}

			log.Printf("✅ [SYNC] Initial full sync COMPLETED in %s", time.Since(start).Round(time.Second))
		})

		if !ok {
			log.Println("⚠️ [SYNC] Initial full sync task DROPPED (sync queue full)")
		}

		// -----------------------------
		// Periodic incremental sync
		// -----------------------------
		// Asset lists: slower cadence (tokens change less often)
		assetTicker := time.NewTicker(12 * time.Hour)
		// Token chains: faster cadence (txns happen constantly)
		chainTicker := time.NewTicker(24 * time.Hour)

		defer assetTicker.Stop()
		defer chainTicker.Stop()

		for {
			select {
			case <-assetTicker.C:
				enqueueAssetListSync()
			case <-chainTicker.C:
				enqueueTokenChainSync()
			}
		}
	}()
}

// enqueueAssetListSync enqueues a lightweight asset-list refresh for RBT/FT/NFT/SC.
func enqueueAssetListSync() {
	ok := EnqueueBackgroundSyncTask(func() {
		start := time.Now()
		log.Println("🔄 [SYNC] Periodic asset-list sync STARTED")

		if err := FetchAndStoreAllRBTsFromFullNodeDB(); err != nil {
			log.Printf("⚠️ [SYNC] FetchAndStoreAllRBTsFromFullNodeDB error: %v", err)
		}
		if err := FetchAndStoreAllFTsFromFullNodeDB(); err != nil {
			log.Printf("⚠️ [SYNC] FetchAndStoreAllFTsFromFullNodeDB error: %v", err)
		}
		if err := FetchAndStoreAllNFTsFromFullNodeDB(); err != nil {
			log.Printf("⚠️ [SYNC] FetchAndStoreAllNFTsFromFullNodeDB error: %v", err)
		}
		if err := FetchAndStoreAllSCsFromFullNodeDB(); err != nil {
			log.Printf("⚠️ [SYNC] FetchAndStoreAllSCsFromFullNodeDB error: %v", err)
		}

		log.Printf("✅ [SYNC] Periodic asset-list sync COMPLETED in %s", time.Since(start).Round(time.Second))
	})

	if !ok {
		log.Println("⚠️ [SYNC] Asset-list sync task DROPPED (sync queue full)")
	}
}

// enqueueTokenChainSync enqueues a heavy get-token-chain sync.
// It uses the sync worker pool, so live updates are not blocked.
func enqueueTokenChainSync() {
	ok := EnqueueBackgroundSyncTask(func() {
		start := time.Now()
		log.Println("🔄 [SYNC] Periodic token-chain sync STARTED")

		if err := FetchAllTokenChainFromFullNode(); err != nil {
			log.Printf("⚠️ [SYNC] FetchAllTokenChainFromFullNode error: %v", err)
		}

		log.Printf("✅ [SYNC] Periodic token-chain sync COMPLETED in %s", time.Since(start).Round(time.Second))
	})

	if !ok {
		log.Println("⚠️ [SYNC] Token-chain sync task DROPPED (sync queue full)")
	}
}
