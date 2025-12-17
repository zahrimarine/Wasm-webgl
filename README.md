# Go WASM Project

This project demonstrates how to compile **Go** code into **WebAssembly (WASM)** and run it in a browser using Go’s WASM runtime.

## Prerequisites

* Go (recommended v1.20 or later)
* Python (for local HTTP server)
* Modern web browser (Chrome / Firefox / Edge)

Check Go installation:

```bash
go version
```

---

## 🛠️ Setup Instructions

###  Create Project Folder

```bash
mkdir go-wasm-drawing
cd go-wasm-drawing
```

The `wasm_exec.js` file is required to run Go WASM in the browser.

```bash
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" .
```

---

### Compile Go to WebAssembly

This command builds the Go source into a WASM binary named `main.wasm`.

```bash
GOOS=js GOARCH=wasm go build -o main.wasm main.go
```

---

###  Run Local Server

WebAssembly must be served via HTTP (cannot be opened via `file://`).

```bash
python -m http.server 8080
```

---

### Open in Browser

Access the application at:

```
http://localhost:8080
```

---



---

Happy coding with Go + WebAssembly 🚀
