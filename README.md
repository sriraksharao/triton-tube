# triton-tube

A distributed video streaming platform written in Go. Videos are uploaded, transcoded to MPEG-DASH via ffmpeg, and served over HTTP. Content storage can be local filesystem or a distributed network of storage nodes using consistent hashing.

## Architecture

```
┌─────────────┐        ┌──────────────────────────────────┐
│   Browser   │◄──────►│         Web Server (cmd/web)      │
└─────────────┘  HTTP  │  - Upload & transcode videos      │
                        │  - Serve DASH manifests & chunks  │
                        │  - Metadata via SQLite or etcd    │
                        └────────────┬─────────────────────┘
                                     │ gRPC
                        ┌────────────▼─────────────────────┐
                        │      Storage Nodes (cmd/storage)  │
                        │  - Consistent hash ring           │
                        │  - Automatic file migration       │
                        └──────────────────────────────────┘
                                     ▲
                        ┌────────────┴─────────────────────┐
                        │    Admin CLI (cmd/admin)          │
                        │  - Add / remove nodes at runtime  │
                        └──────────────────────────────────┘
```

## Prerequisites

- Go 1.24+
- ffmpeg (must be on `$PATH`)
- protoc + protoc-gen-go + protoc-gen-go-grpc (only needed to regenerate protos)

## Building

```bash
# Build all three binaries
go build -o bin/web     ./cmd/web
go build -o bin/storage ./cmd/storage
go build -o bin/admin   ./cmd/admin
```

## Running

### 1. Local filesystem storage (single node)

```bash
./bin/web -port 8080 sqlite metadata.db fs /path/to/video/storage
```

| Argument | Description |
|---|---|
| `sqlite` | Metadata backend (SQLite) |
| `metadata.db` | Path to the SQLite database file |
| `fs` | Content backend (local filesystem) |
| `/path/to/video/storage` | Directory where video chunks are stored |

Open `http://localhost:8080` in your browser to upload and watch videos.

### 2. Distributed network storage

Start one or more storage nodes:

```bash
./bin/storage -host localhost -port 8091 /data/node1
./bin/storage -host localhost -port 8092 /data/node2
```

Start the web server pointing at the storage cluster. The first address is the admin gRPC listener for the web server itself; the remaining addresses are the initial storage nodes:

```bash
./bin/web -port 8080 sqlite metadata.db nw "localhost:9000,localhost:8091,localhost:8092"
```

### 3. Managing storage nodes at runtime

```bash
# Add a new node (triggers automatic file migration)
./bin/admin add localhost:9000 localhost:8093

# Remove a node (migrates its files to remaining nodes first)
./bin/admin remove localhost:9000 localhost:8091

# List current nodes
./bin/admin list localhost:9000
```

## Content Storage Backends

| Backend | Flag | Description |
|---|---|---|
| Filesystem | `fs` | Stores chunks in a local directory tree |
| Network | `nw` | Distributes chunks across storage nodes via consistent hashing |

## Metadata Backends

| Backend | Flag | Description |
|---|---|---|
| SQLite | `sqlite` | Single-node embedded database |
| etcd | `etcd` | Distributed key-value store (for multi-web-server setups) |

## Regenerating Protobuf Code

```bash
make proto
```

This runs `protoc` over `proto/storage.proto` and `proto/admin.proto` and writes generated Go code to `internal/proto/`.

## Project Structure

```
cmd/
  web/      # HTTP server — upload, transcode, serve
  storage/  # gRPC storage node
  admin/    # CLI for managing the storage cluster
internal/
  web/      # Server logic, metadata services, content backends
  storage/  # gRPC storage server implementation
  proto/    # Generated protobuf/gRPC code
proto/
  storage.proto   # VideoContentService (read/write/delete/list)
  admin.proto     # VideoContentAdminService (add/remove/list nodes)
```
