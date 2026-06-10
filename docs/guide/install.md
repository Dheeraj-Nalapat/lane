---
description: Install lane, a single static Go binary with no runtime dependencies. Prebuilt releases, a one-line install script, go install, or build from source on Linux, macOS, and Windows.
---

# Install

lane is a single static Go binary with no runtime dependencies. Pick whichever
method suits your platform — they all leave a `lane` executable on your `PATH`.

You also need, at runtime: **Docker** (≥ 28) with **Compose** (≥ 2.20), **git**,
and — only for Tilt projects — **Tilt**. Run `lane doctor` after installing to
check the environment.

## 1. Prebuilt binary (Linux · macOS · Windows)

Download the archive for your OS/arch from the
[latest release](https://github.com/Dheeraj-Nalapat/lane/releases/latest),
extract it, and move `lane` onto your `PATH`. For example, on Linux/macOS:

```bash
# pick the asset matching your platform, e.g. lane_linux_amd64.tar.gz
curl -sSL https://github.com/Dheeraj-Nalapat/lane/releases/latest/download/lane_linux_amd64.tar.gz \
  | tar -xz
install -m 0755 lane ~/.local/bin/lane   # ensure ~/.local/bin is on PATH
```

## 2. Install script (Linux · macOS)

A one-liner that downloads the right binary for your platform into
`/usr/local/bin`:

```bash
curl -sSL https://github.com/Dheeraj-Nalapat/lane/releases/latest/download/install.sh | sh
```

## 3. `go install` (any platform with Go ≥ 1.22)

```bash
go install github.com/dheeraj-nalapat/lane@latest
# installs to $(go env GOPATH)/bin — add that to PATH
```

## 4. From source

```bash
git clone https://github.com/Dheeraj-Nalapat/lane
cd lane
go build -o ~/.local/bin/lane .
```

## Homebrew

A Homebrew tap is planned but not published yet. On macOS use the prebuilt
binary, the install script, or `go install` above; on Linux, Homebrew isn't the
default — prefer the prebuilt binary or `go install`.

---

Verify:

```bash
lane --version
lane doctor
```
