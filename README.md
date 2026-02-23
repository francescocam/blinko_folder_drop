# blinko-folder-drop

Go service that monitors a folder and pushes files into a self-hosted Blinko instance.

## Behavior
- `.md` / `.markdown`: create one Blinko note from file content.
- Other files: upload as attachment, then create one note linked to that attachment.
- Source file is deleted on success by default.
- On permanent failure, file is moved to `failed/` with a `*.error.json` sidecar.

## Commands
```bash
blinko-folder-drop version
blinko-folder-drop validate-config --config /path/config.yaml
blinko-folder-drop run --config /path/config.yaml
```

## Config
Start from `configs/config.example.yaml`.

Environment overrides:
- `BFD_BASE_URL`
- `BFD_JWT_TOKEN`
- `BFD_INPUT_DIR`
- `BFD_FAILED_DIR`
- `BFD_RECURSIVE`
- `BFD_STABLE_FOR`
- `BFD_SCAN_EVERY`
- `BFD_WORKERS`
- `BFD_MAX_RETRIES`
- `BFD_RETRY_BASE_DELAY`
- `BFD_DELETE_ON_SUCCESS`
- `BFD_ARCHIVE_DIR`
- `BFD_QUEUE_SIZE`
- `BFD_HTTP_TIMEOUT`
- `BFD_LOG_LEVEL`
- `BFD_METRICS_ENABLED`
- `BFD_METRICS_LISTEN_ADDR`

## Linux build
```bash
go build -o blinko-folder-drop ./cmd/blinko-folder-drop
```

Cross build:
```bash
GOOS=linux GOARCH=amd64 go build -o dist/blinko-folder-drop-linux-amd64 ./cmd/blinko-folder-drop
GOOS=linux GOARCH=arm64 go build -o dist/blinko-folder-drop-linux-arm64 ./cmd/blinko-folder-drop
GOOS=windows GOARCH=amd64 go build -o dist/blinko-folder-drop-windows-amd64.exe ./cmd/blinko-folder-drop
```

Systemd unit is in `deploy/systemd/blinko-folder-drop.service`.
