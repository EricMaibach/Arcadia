Phase 0: Core App Runtime Skeleton (Week 1–2)
Goal: Build the foundational execution environment.
Set up App Runtime / Orchestrator:
Minimal runtime to load and execute one WASM module safely.
Internal API for running tools: run_tool(appId, toolName, inputJSON) -> outputJSON.
Implement Registry / Catalog (Postgres or simple JSON storage) to track apps and versions.
Add artifact storage for WASM modules or code bundles.
Test execution: manually deploy one “hello world” app and call it via internal API.
✅ Deliverable: You can deploy and execute an app without any MCP layer.

Phase 1: Manual App Submission via Internal API (Week 3–4)
Goal: Allow apps to be added to runtime without MCP.
Add CLI/internal API for submitting new apps: submit_app(spec.json, code_bundle).
Runtime validates spec minimally (JSON Schema, runtime type).
Hot-load apps into orchestrator for immediate execution.
✅ Deliverable: App generation + deployment is fully functional without AI.

Phase 2: MCP Facade Layer (Week 5–6)
Goal: Introduce MCP as a lightweight, replaceable layer on top of the runtime.
Implement MCP endpoint that:
Maps MCP tool calls to run_tool() in the runtime.
Handles authentication/authorization for AI clients.
Supports discovery: platform.list_apps() and platform.describe_app(appId).
No AI yet; calls come from a human or test AI client.
✅ Deliverable: MCP facade is functional and decoupled; runtime remains unchanged.

Phase 3: AI-Driven App Generation v0 (Week 7–8)
Goal: Let AI generate and submit apps via MCP.
Implement MCP tools for App Generation Service:
appgen.get_spec_template(runtime, capabilities)
appgen.submit_candidate({spec, code_bundle_ref})
MCP layer forwards submissions to runtime/orchestrator for validation + deployment.
Minimal validation: schema checks only, runtime executes the app immediately.
✅ Deliverable: AI can create a new app that appears live in MCP.
Phase 4: Validation & Sandbox Guardrails (Week 9–10)
Goal: Protect runtime and platform from unsafe or buggy apps.
Add sandboxing, timeouts, memory limits for app execution.
Implement policy checks on specs and code (e.g., allowed network egress, forbidden imports).
Add basic logging and error capture at runtime level.
✅ Deliverable: AI-generated apps are safe to run; platform is resilient.
Phase 5: Deployment Pipeline & Versioning (Week 11–12)
Goal: Introduce app lifecycle management.
Build & package apps (WASM or container) before deployment.
Support versioning, rollback, and canary deployments.
Runtime orchestrator maintains multiple app versions, MCP layer continues routing.
✅ Deliverable: Production-ready deployment lifecycle; hot-swappable apps.
Phase 6: Observability & Admin Controls (Week 13–14)
Goal: Operators can monitor and control apps and runtime health.
Add metrics: request rate, error rate, latency per tool.
Implement admin CLI/API to disable, pause, or remove apps.
Add health endpoints for runtime and MCP layer.
✅ Deliverable: Platform is observable, manageable, and operational.
Phase 7: Security, Signing, & Supply Chain (Week 15–16)
Goal: Make platform trustworthy and secure.
Implement artifact signing and verification (Sigstore/Cosign).
Generate SBOMs for all apps.
Add policy enforcement: limits on capabilities, network egress, secret access.
Optional: extend runtime to additional languages (Node, Python) or container support.
✅ Deliverable: Secure, auditable, and modular AI-driven app platform.
Development Philosophy
Always have a working runtime first; MCP is optional.
Introduce AI gradually — first manual submission, then AI via MCP.
Build guardrails incrementally: security, logging, metrics, policies added in small steps.
Keep MCP as replaceable and thin; runtime/orchestrator is the “core” that survives any protocol change.