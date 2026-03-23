# Rubix Decentralized Explorer Backend

This repository contains the backend server for the [Rubix Network](https://rubix.net/) decentralized explorer [Rubix Explorer](https://explorer.rubix.net/). It tracks DIDs, tokens, and real-time network transactions.

## Architecture & Features

- **Real-Time Data**: Integrates an IPFS node to subscribe to the `rubix_txns` PubSub topic for live transactions updates.
- **Dynamic Processing**: Utilizes a highly concurrent processor that dynamically scales worker count based on system load.
- **RESTful API**: Exposes endpoints for real-time network statistics (total tokens, DIDs, SCs), specific token/transaction retrieval, and DID holding balances.

## System Data Flow & Persistence

The explorer operates as a real-time reactive system, processing high volumes of decentralized network events through a highly concurrent pipeline.

### 1. High-Level Data Flow

```mermaid
graph TD
    A[Rubix Network] -- PubSub: rubix_txns --> B[IPFS PubSub Client]
    B -- Raw JSON/Base64 --> C[processor.TxnCallBack]
    C -- EventTransaction --> D[processor.HandleIncomingTxn]
    D -- Enqueue --> E[Dynamic Worker Pool]
    E -- ProcessDBTransaction --> F[Database Persistence]
    
    subgraph "Persistence Layer (database/operations)"
        F --> G[SaveEventTransaction]
        F --> H[SaveTransactionDetails]
        F --> I[ProcessTransactionAssets]
        I --> J[Update Tokens Table]
        I --> K[Update TokenChain/Provenance]
        I --> L[Update DIDBalances/Analytics]
    end
```

### 2. Processing Stages

#### Ingestion & Orchestration
- **PubSub Ingestion**: The server subscribes to the `rubix_txns` topic. Incoming messages are decoded (Base64/JSON) and unmarshaled into the `EventTransaction` model.
- **Validation**: `HandleIncomingTxn` performs strict regex-based validation on all DIDs and TokenIDs before any database interaction occurs.
- **Dynamic Worker Pool**: Validated transactions are enqueued into a worker pool that dynamically scales based on the current workload, ensuring the system remains responsive during traffic spikes.

#### Database Persistence (Persistence Logic)
- **Atomic Transactions**: All asset updates (Tokens, Provenance, and Balances) are performed within a single database transaction to ensure data integrity.
- **Flattening**: The complex, nested transaction payload is simplified into a flattened `TransactionInfo` structure for fast frontend filtering and search.
- **Provenance Handling**: The `TokenChain` and `TokenChainArray` tables work together to maintain a logically sequenced history of every token's movement across the network.
- **Real-Time Analytics**: The `DIDBalances` table is updated incrementally (increments/decrements) during each transaction, allowing for instant "Top Holders" lookups without expensive full-table scans.



## Running the Server

Configure your `.env` (see `.env.example`).

### Development (`go run`)

**MainNet (Default)**
```bash
go run .
```

**TestNet**
```bash
go run . -testnet
```

**Custom Network (Private)**
```bash
go run . -swarmkey config/custom_swarm.key
```

### Production (`go build`)

Build the executable first:
```bash
go build -o explorer.exe .
```

**MainNet (Default)**
```bash
./explorer.exe
```

**TestNet**
```bash
./explorer.exe -testnet
```

**Custom Network (Private)**
```bash
./explorer.exe -swarmkey config/custom_swarm.key
```

## API Reference

All requests follow standard REST principles. Pagination parameters `limit` (default 10) and `page` (default 1) are supported on list endpoints.

### 1. Unified Search

- **`GET /api/get-info?id=<string>`**
  - **Description**: Routes queries based on format (DID, Token, or Transaction).
  - **Input**: `id` - DID, TokenID, or TransactionID.
  - **Output Example (DID)**:
    ```json
    {
      "type": "DID",
      "data": [
        {
          "did": "bafybm...",
          "asset_type": "RBT",
          "token_name": "RBT",
          "balance": 10.5,
          "last_update": 1679567400
        }
      ]
    }
    ```

### 2. Network Statistics (Counts)
- **`GET /api/get-rbt-count`** -> `{ "all_rbt_count": 1250 }`
- **`GET /api/get-ft-count`** -> `{ "all_ft_count": 5400 }`
- **`GET /api/get-nft-count`** -> `{ "all_nft_count": 320 }`
- **`GET /api/get-sc-count`** -> `{ "all_sc_count": 45 }`
- **`GET /api/get-txn-count`** -> `{ "all_transaction_count": 89000 }`
- **`GET /api/get-did-count`** -> `{ "all_did_count": 1200 }`

### 3. Lists and Holders

- **`GET /api/get-latest-transactions?limit=2`**
  - **Output Example**:
    ```json
    [
      {
        "transaction_id": "Qm...",
        "initiator": "bafybm...",
        "owner": "bafybm...",
        "epoch": 1679567400,
        "tokens": { "rbt": [...] },
        "memo": "Payment for services"
      }
    ]
    ```

- **`GET /api/get-did-with-most-rbts?limit=2`**
  - **Output Example**:
    ```json
    [
      {
        "did": "bafybm...",
        "balance": 500.75,
        "last_update": 1679567400
      }
    ]
    ```

- **`GET /api/get-rbt-list?limit=2`**
  - **Output Example**:
    ```json
    [
      {
        "token_id": "Qm...",
        "token_value": 1.0,
        "did": "bafybm...",
        "token_status": 1
      }
    ]
    ```

- **`GET /api/get-ft-group-list`**
  - **Output Example**:
    ```json
    [
      {
        "ftName": "RubixPoints",
        "count": 1000,
        "creatorDID": "bafybm..."
      }
    ]
    ```

- **`GET /api/get-ft-list-by-ftname?ftName=RubixPoints`**
  - **Output**: List of individual FT tokens matching the name.

- **`GET /api/get-sc-list`**
  - **Output**: List of Smart Contract tokens (`token_type: 4`).

### 4. Details and History

- **`GET /api/get-transaction-info?transactionID=<id>`**
  - **Output Example**:
    ```json
    {
      "transaction_id": "Qm...",
      "initiator": "bafybm...",
      "owner": "bafybm...",
      "epoch": 1679567400,
      "quorums": [...],
      "memo": "Initial mint"
    }
    ```

- **`GET /api/get-token-info?tokenID=<id>`**
  - **Output Example**:
    ```json
    {
      "token_id": "Qm...",
      "token_value": 1.0,
      "did": "bafybm...",
      "token_status": 1,
      "token_type": 1
    }
    ```

- **`GET /api/get-transaction-info-list?tokenID=<id>`**
  - **Description**: Returns the full transaction history for a specific token.
- **`GET /api/get-transaction-id-list?tokenID=<id>`**
  - **Output Example**:
    ```json
    [
      {
        "id": 1,
        "token_id": "Qm...",
        "transaction_id": "txn_123",
        "previous_transaction_id": ""
      }
    ]
    ```
