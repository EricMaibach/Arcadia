# Arcadia

Arcadia is an application engine that lets you submit Rust source code, compiles it to
server-side WebAssembly, and runs it in a sandboxed runtime with database access — no
deployment pipeline required. It's paired with an AI module (document search, image
captioning, audio transcription) and an MCP server so an AI agent can develop and run
WASM apps against it directly.

## How it works

1. Implement the `ArcadiaApp` trait in Rust (`initialize`, `handle_tool`,
   `get_available_tools`) and describe your app's tools as JSON.
2. Submit it to the engine over HTTP (or via the MCP server, so an AI agent can write
   and register apps on its own).
3. Arcadia compiles it to WASM, registers its tools, and runs it in a sandboxed runtime
   with access to a per-app SQLite-backed database.
4. Tools can be invoked directly, or scheduled to run on a recurring basis.

## Components

| Component | Description |
|---|---|
| `appengine/` | Go backend: WASM compiler/runtime, app registry, scheduler, HTTP API |
| `appengine/modules/ai` | AI provider abstraction (OpenAI, Ollama) with conversation context, tool calling, and usage tracking |
| `appengine/modules/documents` | Document ingestion and search — PDF/audio/image processing, chunking, and vector storage |
| `mcpserver/` | Model Context Protocol server exposing app submission and management as MCP tools |
| `webadmin/` | React admin UI for managing registered apps and schedules |
| `rustwasmexample/` | Example `ArcadiaApp` implementation |

## AI & document search

- Pluggable providers (OpenAI, Ollama) behind a common interface, with conversation
  context management, tool calling, and token/usage tracking.
- Documents (PDF, audio, images) are parsed via [Apache Tika](https://tika.apache.org/),
  transcribed with Whisper, and captioned, then chunked and embedded into
  [Qdrant](https://qdrant.tech/) for semantic search.
- Structured logging (`log/slog`) with a dedicated log stream for AI request/response
  content, separate from application logs.

## Getting started

The `.devcontainer/` setup brings up the full stack (App Engine, Qdrant, Tika, Ollama,
Whisper) via Docker Compose — open the repo in VS Code / any devcontainer-compatible
editor and it configures itself. Copy `.devcontainer/.env.example` to `.env` and fill
in an OpenAI API key if you want to use OpenAI instead of the local Ollama models.

To run the backend directly:

```bash
cd appengine
go run main.go
```

The server listens on `:8080` by default (see `appengine/config.json`).

To run the admin UI:

```bash
cd webadmin
npm install
npm start
```

## Documentation

- [`appengine/modules/documents/README.md`](appengine/modules/documents/README.md) —
  document module architecture
- [`appengine/TRAIT_USAGE_EXAMPLE.md`](appengine/TRAIT_USAGE_EXAMPLE.md) — how to
  implement `ArcadiaApp`
- [`shared/create_app_description.md`](shared/create_app_description.md) — app
  submission format
- [`webadmin/ADMIN_README.md`](webadmin/ADMIN_README.md) — admin UI features
- [`mcpserver/README.md`](mcpserver/README.md) — MCP tools for app development
- [`docs/`](docs/) — API testing notes and development history
