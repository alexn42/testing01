# Install guide

Covers the compiled `mediasearch` binary on Windows and Debian/Ubuntu
Linux: installing its three runtime dependencies, placing the binary, and
verifying everything works. See [README.md](README.md) for usage and
[building from source](#building-from-source) below if you'd rather compile
it yourself than use a pre-built binary.

## What you need

The binary itself has no dependencies (no cgo, statically linked) — but it
**shells out** to three external tools at runtime, so these must be
installed and on `PATH` on the machine that *runs* mediasearch (not needed
just to download/place the binary):

| Tool | Needed for |
|---|---|
| `tesseract` | OCR (`--text` search on images, video frames, and scanned PDF pages) |
| `ffmpeg` | Video input |
| `pdftoppm`, `pdftotext`, `pdfinfo` (all from **poppler**) | PDF input |

If you only ever search plain images for text, you need just `tesseract`.
If you only do template-image search on plain images, you need none of
these three at all — install them once you hit a case that needs them, or
install all three up front to be safe.

---

## Windows

### 1. Install the three tools

The easiest path is [winget](https://learn.microsoft.com/windows/package-manager/winget/)
(built into Windows 10/11). Open PowerShell:

```powershell
winget install Gyan.FFmpeg
winget install oschwartz10612.Poppler
```

Tesseract has no official winget package, so install it manually:

1. Download the installer from the
   [UB-Mannheim Tesseract wiki](https://github.com/UB-Mannheim/tesseract/wiki)
   (`tesseract-ocr-w64-setup-*.exe`).
2. Run it. **Check "Add to PATH"** if the installer offers it; if not, add
   the install directory (typically `C:\Program Files\Tesseract-OCR`) to
   your `PATH` manually (Settings → System → About → Advanced system
   settings → Environment Variables → edit `Path`).

`winget install`ed packages are normally added to `PATH` automatically. If
`ffmpeg`/`pdftoppm` aren't found after installing, open a **new** terminal
window (PATH changes don't apply to already-open shells) — or, if still not
found, locate their install directory (winget installs under
`%LOCALAPPDATA%\Microsoft\WinGet\Packages\...`) and add it to `PATH`
manually via the same Environment Variables dialog as above.

### 2. Place the binary

Put `mediasearch-windows-amd64.exe` wherever you like — there's no
installer, it's a single file. Two common options:

- Run it by its full path each time, e.g. `C:\tools\mediasearch.exe ...`.
- Put it in a folder that's already on `PATH` (or create one, e.g.
  `C:\tools`, and add it to `PATH` the same way as step 1) so you can just
  type `mediasearch.exe` from anywhere. Rename it to `mediasearch.exe` if
  you'd like a shorter command.

### 3. Verify

Open a **new** PowerShell or Command Prompt window and check each tool
answers:

```powershell
ffmpeg -version
pdftoppm -v
tesseract --version
mediasearch.exe --help
```

If any of those says "not recognized," that tool isn't on `PATH` yet —
revisit step 1.

### 4. Try it

```powershell
mediasearch.exe --input C:\path\to\photo.png --text "hello"
```

You should get a JSON object back with a `"found"` field.

---

## Debian / Ubuntu Linux

### 1. Install the three tools

```bash
sudo apt-get update
sudo apt-get install -y ffmpeg poppler-utils tesseract-ocr
```

### 2. Place the binary

```bash
chmod +x mediasearch-linux-amd64
```

Then either run it by path (`./mediasearch-linux-amd64 ...`), or install it
onto your `PATH` so you can just type `mediasearch`:

```bash
sudo mv mediasearch-linux-amd64 /usr/local/bin/mediasearch
```

### 3. Verify

```bash
ffmpeg -version
pdftoppm -v
tesseract --version
mediasearch --help   # or ./mediasearch-linux-amd64 --help if not on PATH
```

### 4. Try it

```bash
mediasearch --input /path/to/photo.png --text "hello"
```

You should get a JSON object back with a `"found"` field.

---

## Building from source

Needs [Go](https://go.dev/dl/) 1.24 or later. From the `go/` directory:

```bash
go build -o mediasearch ./cmd/mediasearch          # native build
```

Cross-compile for another platform from anywhere (no cgo, so this always
works without needing that platform's toolchain):

```bash
GOOS=windows GOARCH=amd64 go build -o mediasearch.exe ./cmd/mediasearch
GOOS=linux   GOARCH=amd64 go build -o mediasearch-linux ./cmd/mediasearch
GOOS=darwin  GOARCH=arm64 go build -o mediasearch-mac ./cmd/mediasearch
```

The three runtime tools (ffmpeg/poppler/tesseract) are still required at
run time regardless of how the binary was built — see above.

---

## Troubleshooting

**`required tool 'X' was not found on PATH (see README for runtime dependencies)`**
mediasearch tried to run `ffmpeg`, `pdftoppm`, `pdftotext`, `pdfinfo`, or
`tesseract` and couldn't find it. Install the tool that owns it (table
above) and make sure it's on `PATH` — open a **new** terminal after
installing, since existing terminals don't pick up PATH changes.

**Windows SmartScreen blocks the .exe ("Windows protected your PC")**
Expected for an unsigned, freshly built binary — Windows flags any .exe it
hasn't seen before and that isn't code-signed. Click "More info" → "Run
anyway" if you trust the source you got it from. Signing it would remove
this warning but is a separate step not covered here.

**Linux: `Permission denied` when running the binary**
It's not marked executable yet: `chmod +x mediasearch-linux-amd64`.

**Linux: `bash: mediasearch: command not found` after moving it to `/usr/local/bin`**
Either it wasn't actually moved there (check `ls /usr/local/bin/mediasearch`),
or `/usr/local/bin` isn't on your `PATH` (check `echo $PATH`) — unusual on
a standard Debian/Ubuntu setup, but possible in a minimal or custom shell.

**A PDF or video search seems to hang or take a long time**
Expected for large inputs to some degree (OCR and PDF rendering aren't
instant), but if it's *far* longer than expected: for video, raise
`--frame-interval` to sample fewer frames; for large template-image
searches, lower `--match-max-dim` (see `go/README.md` for why that's a
speed/precision tradeoff). A hang with no CPU activity at all more likely
means one of the three external tools itself is stuck — check with your
OS's process monitor.
