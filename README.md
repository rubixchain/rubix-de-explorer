# Rubix Decentralized Explorer Backend

This repository contains the backend server for the [Rubix Network](https://rubix.net/) decentralized explorer [Rubix Explorer](https://explorer.rubix.net/). It tracks DIDs, tokens, and real-time network transactions.

## Architecture & Features

- **Real-Time Data**: Integrates an IPFS node to subscribe to the `rubix_txns` PubSub topic for live transactions updates.
- **Dynamic Processing**: Utilizes a highly concurrent processor that dynamically scales worker count based on system load.
- **RESTful API**: Exposes endpoints for real-time network statistics (total tokens, DIDs, SCs), specific token/transaction retrieval, and DID holding balances.


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
