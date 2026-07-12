<div align="center">
  <img src="web/templates/logo.png" alt="Shrinkray" width="128" height="128">
  <h1>Shrinkray</h1>
  <p><strong>Simple video transcoding for Unraid</strong></p>
  <p>Select a folder. Pick a preset. Shrink your media library.</p>

  ![Go](https://img.shields.io/badge/go-1.24+-00ADD8?style=flat-square&logo=go)
  ![License](https://img.shields.io/badge/license-MIT-green?style=flat-square)
  ![Docker](https://img.shields.io/badge/docker-ghcr.io-2496ED?style=flat-square&logo=docker)
</div>

---

## About This Fork

This is a community fork of [gwlsn/shrinkray](https://github.com/gwlsn/shrinkray), the excellent video transcoding tool created by [@gwlsn](https://github.com/gwlsn).

Both projects share the same core goal: make video transcoding simple and accessible. This fork is streamlined for local-network deployments: authentication and outbound notification services (Pushover, ntfy) were removed to reduce attack surface and external dependencies. The focus is on VAAPI reliability, performance, and a clean, single-purpose codebase.

### Which Should You Use?

| Use Case | Recommendation |
|----------|----------------|
| Intel Arc GPU (A380, A770, B580) | **This fork** — extensive VAAPI fixes |
| AMD GPU on Linux | **This fork** — improved VAAPI support |
| Want the simplest local setup | **This fork** — no auth or external services to configure |
| Need authentication (OIDC/password) | [Original by @jesposito](https://github.com/jesposito/shrinkray) — full auth support |
| Want Pushover or ntfy notifications | [Original by @jesposito](https://github.com/jesposito/shrinkray) |
| Prefer mature, stable release | [Original](https://github.com/gwlsn/shrinkray) |

Both versions include: GPU acceleration, scheduling, quality controls, batch selection, and smart skipping.

---

## Features

- **Smart Presets** — HEVC compress, AV1 compress, 1080p downscale, 720p downscale
- **Full GPU Pipeline** — Hardware decoding AND encoding (frames stay on GPU)
- **Batch Selection** — Select entire folders to transcode whole seasons at once
- **Scheduling** — Restrict transcoding to specific hours (e.g., overnight only)
- **Quality Control** — Adjustable CRF for fine-tuned compression
- **Smart Skipping** — Automatically skips files already in target codec/resolution
- **CPU Fallback** — Optional automatic retry with software encoding
- **Local Only** — No authentication or outbound network calls by design

---

## Quick Start

### Unraid (Community Applications)

1. Search **"Shrinkray"** in Community Applications, or add manually:
   - **Repository**: `ghcr.io/noideadeveloper/shrinkray:latest`
   - **WebUI**: `8080`
   - **Volumes**: `/config` → appdata, `/media` → your media library
2. For GPU acceleration, pass through your GPU device (see [Hardware Acceleration](#hardware-acceleration))
3. Open the WebUI at port **8080**

### Docker Compose

```yaml
services:
  shrinkray:
    image: ghcr.io/noideadeveloper/shrinkray:latest
    container_name: shrinkray
    ports:
      - 8080:8080
    volumes:
      - /path/to/config:/config
      - /path/to/media:/media
      - /path/to/fast/storage:/temp  # Optional: SSD for temp files
    environment:
      - PUID=99
      - PGID=100
    restart: unless-stopped
```

### Docker CLI

```bash
docker run -d \
  --name shrinkray \
  -p 8080:8080 \
  -e PUID=99 \
  -e PGID=100 \
  -v /path/to/config:/config \
  -v /path/to/media:/media \
  ghcr.io/noideadeveloper/shrinkray:latest
```

---

## Presets

| Preset | Codec | Description | Typical Savings |
|--------|-------|-------------|-----------------|
| **Compress (HEVC)** | H.265 | Re-encode to HEVC | 40–60% smaller |
| **Compress (AV1)** | AV1 | Re-encode to AV1 | 50–70% smaller |
| **1080p** | HEVC | Downscale 4K → 1080p | 60–80% smaller |
| **720p** | HEVC | Downscale to 720p | 70–85% smaller |

All presets copy audio and subtitles unchanged (stream copy).

---

## Hardware Acceleration

Shrinkray automatically detects and uses the best available hardware encoder—no configuration required, just pass through your GPU.

### Supported Hardware

| Platform | Requirements | Docker Flags |
|----------|--------------|--------------|
| **NVIDIA (NVENC)** | GTX 1050+ / RTX series | `--runtime=nvidia --gpus all` |
| **Intel Quick Sync** | 6th gen+ CPU | `--device /dev/dri:/dev/dri` |
| **Intel Arc** | Arc A-series / B-series | `--device /dev/dri:/dev/dri` + see below |
| **AMD (VAAPI)** | Polaris+ GPU on Linux | `--device /dev/dri:/dev/dri` |
| **Apple (VideoToolbox)** | Any Mac (M1/M2/M3 or Intel) | Native (no Docker) |

### Intel Arc GPU Setup

Intel Arc GPUs (A380, A770, B580, etc.) require specific configuration:

```bash
docker run -d \
  --name shrinkray \
  --device /dev/dri:/dev/dri \
  --group-add render \
  -e LIBVA_DRIVER_NAME=iHD \
  -e PUID=99 \
  -e PGID=100 \
  -p 8080:8080 \
  -v /path/to/config:/config \
  -v /path/to/media:/media \
  ghcr.io/noideadeveloper/shrinkray:latest
```

Key settings:
- `--device /dev/dri:/dev/dri` — Pass GPU device to container
- `--group-add render` — Add render group permissions (may need GID like `105`)
- `-e LIBVA_DRIVER_NAME=iHD` — Intel Arc requires the iHD driver

Verify GPU access:
```bash
docker exec -it shrinkray vainfo
```

### AV1 Hardware Requirements

AV1 hardware encoding requires newer GPUs:

| Platform | Minimum Hardware |
|----------|------------------|
| **NVIDIA** | RTX 40 series (Ada Lovelace) |
| **Intel** | Arc GPUs, 14th gen+ iGPUs |
| **Apple** | M3 chip or newer |
| **AMD** | RX 7000 series (RDNA 3) |

### VAAPI Troubleshooting

| Error | Cause | Fix |
|-------|-------|-----|
| Exit code 218 mid-encode | 10-bit/HDR format mismatch | Update to latest version (auto-detects bit depth) |
| "auto_scale_0" filter error | Missing VAAPI filter chain | Update to latest version (fixed) |
| "Cannot open DRM render node" | No GPU access | Add `--device /dev/dri:/dev/dri` |
| "vaInitialize failed" | Wrong driver | Set `LIBVA_DRIVER_NAME=iHD` for Intel Arc |
| "Permission denied" | Render group missing | Add `--group-add render` or GID |
| MPEG4/XVID decode failures | Legacy codec VAAPI issue | Update to latest version (fixed) |

---

## Scheduling

Restrict transcoding to specific hours to reduce system load during the day.

1. Open **Settings** (gear icon)
2. Enable **Schedule transcoding**
3. Set start and end hours (e.g., 22:00 – 06:00 for overnight)

**Behavior:**
- Jobs can always be added to the queue
- Transcoding only runs during the allowed window
- Running jobs complete even if the window closes
- Jobs automatically resume when the window reopens

---

## Configuration

Config is stored in `/config/shrinkray.yaml`. Most settings are available in the WebUI.

| Setting | Default | Description |
|---------|---------|-------------|
| `media_path` | `/media` | Root directory to browse |
| `temp_path` | *(empty)* | Fast storage for temp files (SSD recommended) |
| `original_handling` | `replace` | `replace` = delete original, `keep` = rename to `.old` |
| `subtitle_handling` | `convert` | `convert` or `drop` unsupported subtitles |
| `workers` | `1` | Concurrent transcode jobs (1–6) |
| `quality_hevc` | `0` | CRF override for HEVC (0 = default, 15–40) |
| `quality_av1` | `0` | CRF override for AV1 (0 = default, 20–50) |
| `schedule_enabled` | `false` | Enable time-based scheduling |
| `schedule_start_hour` | `22` | Hour transcoding may start (0–23) |
| `schedule_end_hour` | `6` | Hour transcoding must stop (0–23) |
| `allow_software_fallback` | `false` | Retry failed GPU encodes with CPU |
| `keep_larger_files` | `false` | Keep transcoded output even if larger than source |
| `hide_processing_tmp` | `false` | Hide `shrinkray.tmp` working files from the file browser |
| `log_level` | `info` | Logging verbosity: `debug`, `info`, `warn`, `error` |
| `layout_design` | `split` | UI layout: `split` (side-by-side) or `tabs` |
| `ffmpeg_path` | `ffmpeg` | Path to ffmpeg binary |
| `ffprobe_path` | `ffprobe` | Path to ffprobe binary |

### Environment Variables

All settings can be overridden with environment variables using the `SHRINKRAY_` prefix:

```bash
SHRINKRAY_WORKERS=2
SHRINKRAY_ALLOW_SOFTWARE_FALLBACK=true
```

---

## CPU Fallback

By default, if a GPU encode fails, the job fails with a clear error message. This is intentional—GPU encodes should succeed on properly configured systems.

Enable **"Allow CPU encode fallback"** only if:
- A small number of files fail due to unusual codecs
- You want those files transcoded anyway, even if slower
- You've verified your GPU is working correctly for other files

When enabled, failed GPU encodes automatically retry with software encoding.

---

## Building from Source

```bash
git clone https://github.com/NoIdeaDeveloper/shrinkray.git
cd shrinkray

go build -o shrinkray ./cmd/shrinkray
./shrinkray -media /path/to/media
```

**Requirements:** Go 1.24+, FFmpeg (with HEVC/AV1 support), Node.js 22+ (for E2E tests)

### Running Tests

```bash
# Go unit tests
go test ./...

# E2E tests (Playwright)
# The Shrinkray server must be running on :8080 before tests start.
npm install && npx playwright install
go build -o shrinkray ./cmd/shrinkray
mkdir -p /tmp/test-media
./shrinkray -media /tmp/test-media &
npm test
```

---

## Acknowledgments

This project is built on the excellent work of [@gwlsn](https://github.com/gwlsn) and the original [shrinkray](https://github.com/gwlsn/shrinkray) project, and the [@jesposito](https://github.com/jesposito) fork which added VAAPI fixes, authentication, and notifications. Thank you for making these tools open source.

Additional contributions from [@akaBilih](https://github.com/akaBilih) for the tabbed layout feature.

---

## License

MIT License — see [LICENSE](LICENSE) for details.
