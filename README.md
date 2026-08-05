<p align="center">
  <img alt="raft" src="https://shieldcn.dev/header/surface.svg?title=raft&subtitle=CLI+for+transferring+files+directly+between+two+devices+over+TCP.&mode=dark&font=space-grotesk">
</p>

<p align="center">
  <picture>
    <img alt="demo gif" src="/demo.gif">
  </picture>
</p>

---

A small CLI for transferring files directly between two machines over TCP.

Run `share` on one machine, `receive` on the other, and the file streams straight from sender to receiver. No cloud upload, no accounts, and no file size limits.

Transfers can be paused while they're running and resumed after a disconnect without retransferring data that's already been verified.


## Quick start

### Send a file

```bash
./raft-linux-amd64 share ./movie.mkv
```

This starts listening on port `9876` and prints an address such as:

```text
Listening on:

<senders-ip>:9876
```

Send that address to the receiver.

### Receive a file


```bash
./raft-linux-amd64 receive <senders-ip>:9876 ./downloads
```

If the output directory is omitted, the current directory is used.

- On Windows, use `raft-windows-amd64.exe`.

- On macOS (Apple Silicon), use `raft-darwin-arm64`.

During a transfer, the receiver can control the sender without opening another connection.

* `p` + Enter pauses the transfer.
* `r` + Enter resumes it.

## Installation

Download the binary for your operating system from the project's Releases page [here](https://github.com/literallyrounak/raft/releases).

No Go installation is required.

## Features

* Direct peer-to-peer file transfer over a single TCP connection
* Resume interrupted transfers from the last verified chunk
* Pause and resume an active transfer from the receiver
* SHA-256 verification for every 4 MB chunk
* Prebuilt binaries for Linux, macOS, and Windows

## How it works

The sender opens a TCP listener and waits for a receiver to connect.

Once connected, the sender scans the file in 4 MB chunks, computes a SHA-256 hash for each chunk, and sends a manifest containing:

* filename
* file size
* chunk hashes

The receiver compares that manifest with any existing partial transfer and tells the sender which chunk index to begin streaming.

Chunks are then transferred in order over the same TCP connection. Every chunk is verified before it's written to disk, so corruption is detected immediately instead of after the transfer finishes.

## Requirements

Both machines must be able to reach each other on the chosen TCP port (default `9876`).

On the same LAN, this usually means allowing the connection through the operating system's firewall.

Some mobile hotspots isolate connected devices from each other ("AP isolation" or similar). If the receiver cannot connect, check whether client isolation is enabled.

## Limitations

* Transfers one file at a time.
* No NAT traversal or relay server.
* Connections are currently unencrypted. For untrusted networks, tunnel the connection through SSH or a VPN.

## Project structure

```text
raft/
├── main.go
├── internal/
│   ├── protocol/
│   └── transfer/
|   └── ui/
├── go.mod
├── demo.gif
└── README.md
```

* `main.go`                         - CLI entry point (`share` / `receive`)
* `internal/protocol`               - wire format and message framing
* `internal/transfer/sender.go`     - sender implementation
* `internal/transfer/receiver.go`   - receiver implementation
* `internal/transfer/state.go`      - resume state persistence
* `internal/transfer/gate.go`       - pause/resume synchronization
* `internal/ui/model.go`            - bubbletea models & lipgloss styles

## Why I built it

I wanted something simpler than uploading large files somewhere, starting a temporary HTTP server, or restarting an entire transfer after a dropped connection.

`raft` is intentionally small: one sender, one receiver, one TCP connection, and just enough protocol to make interrupted transfers recoverable.
