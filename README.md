# badserv

This HTTP server is designed to test HTTP clients by forcing the server to perform certain actions based on the 'action' query parameter passed by the client.

## Getting Started

Install from source: `go install -v -trimpath github.com/ninedraft/badserv`

Or download the latest release from the [releases page](https://github.com/ninedraft/badserv/releases).

## Flags:
- -http: address to serve HTTP requests (default "localhost:7080")
- -log-level: log level, default: INFO

## Usage

Just call the server on any path with `action` query parameter.

```bash
curl http://localhost:7080/?action=hang
```

The server supports the following actions:

- hang: The server will hang on request until the client closes the connection.
- close: The server will close the connection without an HTTP response.
- slow-write: The server will write the response slowly, byte by byte, at a rate of 10 bytes per second.
