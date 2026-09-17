# Arcadia

A local platform where the AI builds its own applications. It writes them, Arcadia compiles them to WebAssembly and runs them under an explicit capability model, and they become tools the AI can call.

Everything runs on your machine. Built in 2025.

---

## The idea

This started from a guess about where computing goes: the interface becomes the AI. You talk or type, and most of the screen furniture falls away.

But the apps don't go away. The AI still needs somewhere to keep data, work that runs on a schedule, and logic that behaves the same way every time. A model isn't those things. It doesn't persist and it isn't deterministic.

So the AI needs apps. And the AI can write code. Let it write its own.

The question that follows isn't whether the code is any good. It's what the code is allowed to do. An agent that extends itself has to be granted something, and "access to your computer" is too blunt an answer — it's one permission covering every future app, granted before you know what any of them will be.

So an app gets no ambient authority at all. It runs in a WASM sandbox with a fixed set of imports and nothing else. No filesystem, no network, no processes. It can only call what the platform explicitly brokers.

The shorthand in my head was an operating system with the AI as the shell — apps, storage, scheduling, and a kernel that decides what each app can touch.

The loop:

1. The AI submits a Rust trait implementation through an MCP tool (`create_app`)
2. Arcadia validates it, compiles it to WebAssembly, and stores the artifact
3. The app registers its tools in a registry
4. Those tools become available to the AI over MCP, callable like any other tool
5. When one is called, it runs in the sandbox with only the capabilities the host hands it

Compiling buys persistence. A generated script is ephemeral. An Arcadia app is a versioned artifact with a declared interface, still there next session, callable by name.

`food-tracker` in the registry is a working example: seven tools, compiled from Rust the AI wrote, persisting to the shared app database.

---

## Architecture

Three deployable pieces:

**AppEngine** (Go) — the platform. Hosts the WASM runtime, brokers capabilities to running apps, manages the app registry and build pipeline, runs the scheduler and file watcher, and owns the document ingestion pipeline.

**MCPServer** (Python) — the MCP surface. Exposes `list_apps`, `create_app`, `run_app`, and `schedule_app_run` to any MCP client.

**WebAdmin** (React) — chat interface, app management, configuration.

### Inside AppEngine

The engine is a modular monolith. Two modules, each owning its domain completely:

```
modules/ai/              modules/documents/
├── facade.go            ├── facade.go
├── module.go            ├── module.go
├── interfaces/          ├── interfaces/
│   ├── core.go          │   ├── core.go
│   └── external.go      │   └── external.go
├── models/              ├── models/
├── core/                ├── core/
├── providers/           ├── detection/
└── tools/               ├── processors/
                         ├── providers/
                         └── stores/
```

Each module declares its public surface in `facade.go` and its external dependencies in `interfaces/external.go`. That second file is the important one. A module defines the `Logger`, `Metrics`, `Database`, and `Cache` interfaces it needs, and the host supplies implementations. The module owns the abstraction, not the provider, so neither module depends on a concrete outside type.

Modules never import each other. When the AI module needs embedding search from the documents module, `main.go` supplies an `EmbeddingSearchAdapter` that bridges the two contracts. `main.go` is the composition root and the only place the two modules meet.

The point of that arrangement is that either module could be lifted out and run separately without touching its internals — the seams are already where they'd need to be. It also keeps the modules independently testable, since everything outside a module reaches it through an interface the module itself declared.

Inside the modules the same preference shows up smaller. Document processors register themselves against a common interface rather than inheriting from a base type. Type detection is a chain of independent detectors run in sequence, not a class hierarchy. The Tika client is wrapped in a circuit breaker rather than subclassed. Composition throughout, which is partly Go's influence and partly how I'd build it anyway.

---

## The capability model

This is the part worth understanding before reading the code, because it explains decisions that look odd otherwise.

**The trust boundary is host versus guest.** Apps are AI-written, so they are untrusted code by design. The goal is that an app can do useful work without being able to touch the machine it runs on.

The enforcement is the host function surface. A WASM module gets exactly four imports and nothing else:

| Import | Purpose |
|---|---|
| `db_query` | Read from the shared app database |
| `db_exec` | Write to the shared app database |
| `db_prepared_query` | Parameterized read |
| `claude_query` | Send a message to the configured AI provider |

No WASI. No filesystem, no network, no process spawning, no clock, no environment. The guest has no ambient authority — it can only reach what those four imports reach.

**Apps share one database on purpose.** They are not isolated from each other, and that is not an oversight. Arcadia is a single-user system where apps are meant to compose — a nutrition app should be able to read what a fitness app wrote. Isolating them would defeat the point. The boundary being defended is the host, not the app.

---

## Document pipeline

The documents module watches configured folders and makes their contents searchable.

Type detection runs in stages, cheapest first, stopping at the first answer: extension, MIME, content, Tika. Processing dispatches through a registry — PDF, text, and Tika processors each implement a common interface and register themselves. PDF includes OCR. The Tika client sits behind a circuit breaker.

Documents are chunked, embedded through Ollama, and stored in Qdrant. Search is exposed to the AI as a tool.

---

## Stack

Go 1.25, wasmtime, SQLite, Qdrant, Ollama, Apache Tika. Apps are written in Rust and compiled to `wasm32`. MCP server in Python, WebAdmin in React.

---

## Repository layout

```
appengine/          Go platform
  main.go           composition root
  modules/          ai, documents
  services/         wasm runtime, scheduler, filewatcher, queue, registry
  handlers/         HTTP
  wasm_wrapper_template.rs   guest-side ABI given to the AI
mcpserver/          Python MCP server
webadmin/           React admin UI
shared/             shared definitions
rustwasmexample/    minimal guest app example
```

`wasm_wrapper_template.rs` is the guest half of the host ABI — the `extern "C"` declarations and the trait the AI implements.

---

## Status

Working end to end. Apps can be created, compiled, registered, scheduled, and called; documents are ingested and searchable.

Personal research project, not maintained for outside use.