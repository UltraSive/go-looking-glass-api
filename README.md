# Go Looking Glass Server

A lightweight, self-hosted **Looking Glass** web API written in Go that allows users to run network diagnostic commands like `ping` and `mtr` (My Traceroute) on demand. Supports both **streaming** and **non-streaming** output modes.

---

## Features

* Run `ping` or `mtr` to test connectivity from your server
* Stream results in real-time using `NDJSON`
* Validate IPv4 and domain targets
* Easy to deploy with Docker
* Written with `chi` and idiomatic Go

---

## 🚀 Quick Start

### Prerequisites

* Go 1.20+
* Docker (optional, for containerized deployment)

### Local Build and Run

```bash
go build -o looking-glass
./looking-glass
```

### 🐳 Docker

#### Build Image

```bash
docker build -t go-looking-glass .
```

#### Run Container

```bash
docker run --rm -p 8080:8080 go-looking-glass
```

---

## API Usage

### `GET /ping`

Ping a target.

**Query Parameters:**

| Param       | Description                 | Required | Example           |
| ----------- | --------------------------- | -------- | ----------------- |
| `target`    | Domain or IPv4 to ping      | ✅        | `8.8.8.8`         |
| `streaming` | Enable streaming via NDJSON | ❌        | `true` or `false` |

**Examples:**

* Non-streaming:

  ```bash
  curl "http://localhost:8080/ping?target=8.8.8.8"
  ```

* Streaming:

  ```bash
  curl "http://localhost:8080/ping?target=8.8.8.8&streaming=true"
  ```

### `GET /mtr`

Run MTR (My Traceroute) with `--raw` and 10 report cycles.

**Query Parameters:**

| Param       | Description                 | Required | Example           |
| ----------- | --------------------------- | -------- | ----------------- |
| `target`    | Domain or IPv4 to trace     | ✅        | `google.com`      |
| `streaming` | Enable streaming via NDJSON | ❌        | `true` or `false` |

**Examples:**

* Non-streaming:

  ```bash
  curl "http://localhost:8080/mtr?target=google.com"
  ```

* Streaming:

  ```bash
  curl "http://localhost:8080/mtr?target=google.com&streaming=true"
  ```

---

## How It Works

* Uses Go's `os/exec` to run `ping` and `mtr`
* Parses the output line-by-line
* Sends back structured JSON or NDJSON if streaming
* Validates input to prevent command injection or malformed requests

---

## 🔐 Security Notes

* Validates IPv4 addresses and domain formats
* Does **not** support IPv6 (yet)
* Make sure to secure your deployment (e.g., with reverse proxy, auth, rate limiting)

---

## Tech Stack

* [Go](https://golang.org/)
* [Chi Router](https://github.com/go-chi/chi)
* `mtr`, `ping` (must be installed in the container or host)

---

## Roadmap

* [ ] Add IPv6 support
* [ ] WebSocket streaming support
* [ ] UI dashboard

---

## Sample Output

**Streaming Ping**

```json
{"type":"ping","seq":1,"rtt_ms":12.7}
{"type":"ping","seq":2,"rtt_ms":12.9}
```

**Non-Streaming MTR**

```json
[
  {"type":"hop","hop":1,"ip":"192.168.1.1"},
  {"type":"ping","hop":1,"rtt":15,"cycle_id":1},
  ...
]
```

---

## License

MIT License. Feel free to use and modify.