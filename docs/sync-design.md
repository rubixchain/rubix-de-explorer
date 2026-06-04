# Explorer fullnode-sync design doc

Status: **draft, awaiting review.** No code changes yet — this document captures
the decisions that must be agreed before implementation work resumes on the
libp2p-paginated sync flow (`/x/<fullnodePeerID>RubixCore/1.0` →
`POST /rubix/v1/fullnode/sync-txn-info-chain`, wire shape
`{token_ids, offset} → {data, has_more, advanced_by}`).

The fullnode side is on `vaishnav/fullnode-sync-API-for-explorer` of
`rubixchain/rubixgoplatform`. Explorer side is `vaishnav/sync` of
`rubixchain/de-explorer`. Implementation will start only after this doc is
signed off.

---

## Topic 1 — Defensive `entry.Info == nil` handling

**Decision: (a) treat as a hard validation failure for that token.**

Same handling as a structural break: log the offending entry ID + token,
set `state.partial = true`, stop processing that token for the rest of the
pagination cycle, leave the token flagged so the next cycle retries.

**Rationale.** A nil `Info` means the fullnode emitted a chain entry with no
payload to interpret. Helpers downstream (`SaveTransactionDetails`,
`ProcessTransactionAssets`, and any future sync-only equivalent) all
dereference `info.Tokens` / `info.Quorums`. The chain past that entry is
unverifiable in the current cycle regardless of cause — we don't know the
correct head to use as `prev` for subsequent entries, and we have no asset
side-effects to apply. Continuing past it would either panic or silently
desynchronise. Crashing the whole sync cycle would block other healthy
tokens in the same batch from completing, so (c) is too aggressive.
Skipping just that entry (b) breaks the structural-validation invariant
that `chain[i].prev == lastSeenID` — the next entry's `prev` would refer
to an ID we never accepted.

The check belongs on the explorer side (the consumer); the fullnode never
intentionally emits null `info`, so the check is defensive against a
server-side FK/integrity bug, not a contract.

## Topic 2 — Sync ingestion path vs. pubsub ingestion path

**Decision: (b) — new sync-only helper that inserts the immutable transaction
rows but does NOT touch `Tokens.*` mutable state.**

Sync's responsibility narrows to *"backfill the transaction history."* Token
head, status, balances, and pledged state remain pubsub's responsibility.
Sync calls a new helper that writes only the rows that can't conflict with
pubsub: `Transactions`, `EventTransaction`, `TransactionInfo` (and any other
purely-immutable per-txn row). It does **not** call `ProcessTransactionAssets`
and does **not** write `Tokens` / `DIDBalance` / `TokenChain` head columns.

**Rationale.** The constraint is that `ProcessTransactionAssets` must not be
modified; the existing flow is the priority path. Option (a) duplicates the
full PTA logic into a sync-only twin, doubling the surface that has to stay
in sync as PTA evolves — the explorer's audit history shows PTA changes
frequently, and a drifted copy would be a long-term liability. Option (c)
adds a mutex/advisory lock and reuses PTA, but the deadlock surface (PTA
takes implicit row locks across `Tokens`, `TokenChain`, `DIDBalance`) is
not worth the operational risk for a background job.

(b) accepts the trade-off that sync alone won't advance
`Tokens.transaction_id`: if pubsub is genuinely silent and we sync from a
flagged-but-stale state, the head pointer won't move forward solely from
sync. The mitigation is the race-aware `MarkTokenSyncedWithStatus` already
in place — it always clears `needs_sync` (the sync attempt completed) and
conditionally updates `token_status` only if pubsub agrees on the head.
The token stays "synced from history" without claiming the pubsub stream
is caught up.

**Open question for review:** if pubsub is silent for an extended period
(weeks, not minutes), does the explorer need a fallback to advance
`Tokens.transaction_id` from sync alone? Today's answer is "no — pubsub
silence is the bigger alarm." Worth confirming with the operations team.

## Topic 3 — `ProcessTransactionAssets` monotonicity audit

**Finding: (a). The existing helper is NOT monotonic.**

`Tokens.TransactionID` is unconditionally overwritten on every processed
entry — verified at
[database/operations/token_operations.go:321](../database/operations/token_operations.go#L321)
(transfer/owner-change path) and
[database/operations/token_operations.go:399](../database/operations/token_operations.go#L399)
(pledge path). Neither call site compares the incoming `txnID` against the
existing `Tokens.TransactionID` before assigning.

The partial defense at
[database/operations/token_operations.go:313](../database/operations/token_operations.go#L313)
flags the token (`NeedsSync = true`) when
`info.PreviousTransactionID != existing.TransactionID` — but it still
proceeds to overwrite the head with the incoming `txnID` afterwards. The
flag fires; the move-backward still happens.

**Impact.** If pubsub processes entry N+1 first (network reordering) then
N arrives, `Tokens.transaction_id` ends up pointing at entry N, not N+1.
`token_status` follows the same path and gets clobbered from the older
entry's role. This is an existing bug independent of sync, but the new
sync flow widens the blast radius: paginated sync ingests in chain order,
but pubsub doesn't, and any overlap between sync's last-page and pubsub's
in-flight stream can move the head backward.

**Proposed fix (separate change, before sync goes live).** Gate the
`TransactionID` / `TokenStatus` / `LatestRole` updates on the new entry
being strictly newer than the current head. The cheap version: keep a
sequence number or `created_at` per chain entry and compare. The
chain-pointer version: only assign when
`info.PreviousTransactionID == existing.TransactionID` (i.e. we are
extending the head, not arriving out of order). When the new entry is
out-of-order, insert the immutable `Transactions` / `TransactionInfo`
rows, flag `NeedsSync = true`, and leave the head pointer alone.

This is a behaviour change to a hot path, so it warrants its own PR with
its own tests — call it out as a prerequisite for the sync rollout, not
folded into the sync PR.

---

## New function signatures (no bodies)

These are the new helpers expected on the explorer side once the
decisions above are accepted. Bodies will be filled in during the
implementation phase.

```go
// database/operations/token_sync_operations.go

// IngestSingleEntry writes the immutable per-transaction rows for one chain
// entry. It does NOT touch Tokens.* mutable state — pubsub remains the only
// writer of Tokens.transaction_id, Tokens.token_status, and DIDBalance.
//
// The caller must:
//   - run this inside the supplied gorm.DB transaction
//   - have already checked TransactionExists returned false for entry.ID
//   - have already validated entry.PreviousTransactionID against the running head
//
// Returns an error if entry or entry.Info is nil; callers translate that into
// a chain-break for the token (topic 1, decision (a)).
func IngestSingleEntry(tx *gorm.DB, txnID string, entry *model.SyncedTxn) error

// MarkTokenSyncedWithStatus clears needs_sync (always) and updates token_status
// only when Tokens.transaction_id still points at lastSeenID — i.e. no
// concurrent writer (typically pubsub) has advanced the head past where sync
// just finished. Already implemented; signature included for completeness.
func MarkTokenSyncedWithStatus(db *gorm.DB, tokenID, lastSeenID string, lastRole int16) error
```

No new helper introduced for "apply transaction effects to Tokens" — sync
deliberately does not own that surface. If topic 2's open question changes
the answer, that helper would be added then.

---

## Minimal change-set on the explorer side

Assuming all three decisions are accepted, the explorer-side implementation
narrows to:

1. **Wire format** — `model.SyncTransactionInfoFromFullnodeRequest`,
   `SyncTransactionInfoFromFullnodeResult`, `SyncTokenChainResponse` with
   `{data, has_more, advanced_by}`. Already in tree.
2. **Client** — `TokenChainClient.FetchPage(req) (SyncTokenChainResponse, error)`
   over libp2p `/x/<peerID>RubixCore/1.0`. Already in tree.
3. **Service pagination loop** — `RunOnce` paginates per batch, per-token
   `tokenPageState{lastSeenID, lastRole, partial, anyEntries}`, structural
   validation via running `lastSeenID`, defensive aborts mark all batch
   tokens partial. Already in tree.
4. **New: `entry.Info == nil` guard inside `IngestSingleEntry`** — returns an
   error; the service's existing partial-on-error flow handles the rest.
5. **`IngestSingleEntry` body** — keep it narrow per topic 2 decision (b).
   Write `EventTransaction`, `Transactions`, `TransactionInfo`. **Do not**
   call `ProcessTransactionAssets`. Already in tree but currently calls PTA
   — this is the one in-place change the decision implies.
6. **No new column.** `needs_sync` remains the only state flag. No retry
   counter, no quarantine column, no `sync_offset` column — pagination
   re-starts at `offset=0` each cycle and `TransactionExists` dedupe (primary
   key lookup, microseconds) is the natural cursor. 3600 already-committed
   entries cost ~10–30 s of dedupe work per recovery cycle, which is
   acceptable for a job that runs on an hours-scale interval.
7. **Scheduling** — sync interval set in hours; service is single-instance
   per process; no mutex needed because the interval is comfortably longer
   than a full `RunOnce` execution. No overlap protection added.
8. **Separate PR (prerequisite for rollout):** monotonicity fix in
   `ProcessTransactionAssets` per topic 3.

Item 5 is the one substantive code change implied by accepting topic 2(b);
the rest is already wired or is configuration / scheduling discipline.

---

## Open questions before coding begins

1. **Topic 2 open question** — pubsub-silent fallback. Confirm with ops that
   "pubsub silence is the louder alarm" is the right framing, or scope a
   follow-up to let sync advance `Tokens.transaction_id` under explicit
   operator opt-in.
2. **Topic 3 monotonicity fix** — is the chain-pointer gate
   (`info.PreviousTransactionID == existing.TransactionID`) the right rule,
   or do we need a true sequence/timestamp comparison? Some entry kinds
   (pledge, unpledge) may not carry a meaningful `previous_transaction_id`
   for the destination token. Audit each role.
3. **`IngestSingleEntry` write set** — the current in-tree implementation
   still calls `ProcessTransactionAssets`. Removing that call is a behaviour
   change for the sync path; the existing tests assert PTA's side-effects
   indirectly (e.g. test seeds working around the PTA-short-circuit caused
   by `minimalInfo`). The test rework needs to be sized before committing
   to (b).
4. **Empty-token handling at the operational level — resolved.** A token
   that returns no entries stays flagged and the next cycle retries the
   same peer. That is the accepted behaviour for the first cut: each
   fullnode hosts only a slice of the network's tokens, so an empty
   response from peer A doesn't mean the chain is gone; we will reach it
   from a different peer once multi-fullnode connection lands. Indefinite
   single-peer retry is acceptable until then — the dedupe path makes
   re-checks cheap, and `needs_sync` keeps the work queued. Multi-peer
   rotation (resolver returning all candidates, per-empty-token fallback,
   terminal `chain_unavailable` only when every peer is exhausted) is
   scoped to the next development phase, not this PR.

---

Decisions and open questions above should be reviewed by both backend and
explorer leads before any of the in-tree code is finalised for merge.
Items 1 and 5 in §"Minimal change-set" are the only ones that block the
sync PR from going green; items 7 and 8 are pre-rollout asks.
