# raft

Direct device-to-device file transfer over TCP. No cloud upload, no size limits, no compression - the file goes straight from one machine to the other.

## How it works

One side runs `share`, which opens a TCP listener and waits for a peer to connect. The other side runs `receive` with that address, which connects directly to it. Once connected, everything happens over that one socket - there's no server or relay involved.

On connect, the sender hashes the file in 4MB chunks (SHA-256) and sends a manifest (filename, size, chunk hashes) to the receiver. The receiver then tells the sender which chunk index to start streaming from. Chunks are streamed in order; the receiver verifies each chunk's hash before writing it to disk, so a corrupted chunk is caught immediately.

Two extra things run alongside the transfer:

- **Pause/resume during an active transfer** — the receiver can type `p` + Enter to pause and `r` + Enter to resume. This is a live signal sent back to the sender over the same connection; the sender just pauses between chunks until told to continue.
- **Resume after a disconnect** — the receiver keeps a small `.p2pstate` file next to the output, tracking the last confirmed chunk. If the connection drops (or the receiver is killed) and you run `receive` again, it picks up from where it left off instead of starting over. The state file is removed once the transfer finishes successfully.

## Project structure

```
raft/
  main.go                        CLI entry point (share / receive subcommands)
  internal/protocol/protocol.go  wire format: message framing, manifest, chunk encoding
  internal/transfer/sender.go    sender: hashes file, streams chunks, handles pause signals
  internal/transfer/receiver.go  receiver: verifies chunks, writes to disk, sends pause/resume
  internal/transfer/state.go     resume state persistence
  internal/transfer/gate.go      pause/resume synchronization
  dist/                          prebuilt binaries (linux, darwin, windows)
```

## Usage

Prebuilt binaries for Linux, macOS, and Windows are in the [Releases](https://github.com/literallyrounak/raft/releases) page - download the one for your OS, no Go installation needed.

**Sending a file:**

```
./raft-linux-amd64 share ./somefile.mp4
```

This prints the address it's listening on (default port `9876`). Share that address with whoever's receiving.

**Receiving a file:**

```
./raft-linux-amd64 receive <sender-ip>:9876 ./downloads
```

The output directory defaults to the current directory if omitted.

On Windows, use `raft-windows-amd64.exe` instead. On macOS, use `raft-darwin-arm64`.

Both sides need to be reachable from each other on the given port - if you're on the same LAN this is usually just a firewall prompt to allow through. Mobile hotspots sometimes isolate connected devices from each other; disable "AP isolation" or similar if a connection times out.

If a transfer is interrupted, just run the same `receive` command again - it'll resume automatically if a partial file is found.