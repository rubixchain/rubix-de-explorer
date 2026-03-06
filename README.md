# Rubix Decentralized Explorer Backend

This repository contains the backend server for the [Rubix Network](https://rubix.net/) decentralized explorer. It tracks Decentralized Identifiers (DIDs), tokens, and real-time network transactions.

## Architecture & Features

- **Real-Time Data**: Integrates an IPFS node to subscribe to the `rubix_txns` PubSub topic for live block updates.
- **Dynamic Processing**: Utilizes a highly concurrent processor that dynamically scales worker count based on system load.
- **RESTful API**: Serves endpoints for aggregated metrics, specific block details, DID analytics, and token ownership.
- **PostgreSQL Storage**: Efficiently flattens and stores complex token chain structures.

## Prerequisites

- Go 1.20+
- PostgreSQL Server
- `.env` file containing database credentials (see `.env.example`).
- `ipfs.exe` binary in the project root.
- A valid private network swarm key file (e.g., `config/custom_swarm.key`).

## Running the Server

Configure your `.env` and run the application with your swarm key.

**Note:** By default, the Explorer connects to **MainNet** if no network flag is specified. To connect to the **TestNet**, you must explicitly pass the `-testnet` flag.

### MainNet (Default)
```bash
go run ./cmd/explorer
```

### TestNet
```bash
go run ./cmd/explorer -testnet -swarmkey config/custom_swarm.key
```

Alternatively, build and execute the binary:

```bash
go build -o explorer.exe ./cmd/explorer
./explorer.exe -testnet -swarmkey config/custom_swarm.key
```
