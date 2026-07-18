# Architecture — shadow-llm-evaluator

This service exposes a synchronous primary LLM API and asynchronously shadow-evaluates a candidate model. The user always gets the primary response immediately; candidate work continues even if the client disconnects.

---

## 1. High-level request flow

```mermaid
flowchart LR
  Client([Client]) -->|POST /v1/primary| Handler[PrimaryHandler]

  Handler -->|1. sync Complete| Primary[Primary Completer]
  Primary --> PrimaryLLM[(Primary LLM / Simulator)]
  PrimaryLLM --> Handler
  Handler -->|2. JSON response immediately| Client

  Handler -.->|3. EvaluateAsync fire-and-forget| Shadow[Shadow Runner]
  Shadow -->|detached context| Candidate[Candidate Completer]
  Candidate --> CandidateLLM[(Candidate LLM / Simulator)]
  CandidateLLM --> Compare[Content Comparator]
  Compare -->|match / mismatch JSON logs| Logs[(Structured logs)]
```

**Steps**

1. Client calls `POST /v1/primary` with chat messages.
2. Handler calls the **primary** Completer synchronously and returns that response.
3. Handler triggers **shadow** evaluation in a background goroutine.
4. Shadow runner calls the **candidate** Completer, compares outputs, and logs mismatches as clean JSON.

---

## 2. Package architecture (SOLID)

```mermaid
flowchart TB
  subgraph entry["main"]
    Main[main.go]
    Cfg[config.Load]
  end

  subgraph http["internal/handler"]
    PH[PrimaryHandler]
  end

  subgraph domain["abstractions"]
    CompIface[llm.Completer]
    EvalIface[shadow.Evaluator]
    CmpIface[compare.Comparator]
  end

  subgraph impl["implementations"]
    HTTP[llm.HTTPClient]
    SimP[simulator.Primary]
    SimC[simulator.Candidate]
    Runner[shadow.Runner]
    ContentCmp[compare.ContentComparator]
  end

  subgraph data["internal/models"]
    DTOs[ChatRequest / ChatResponse]
  end

  Main --> Cfg
  Main --> PH
  Main --> Runner
  PH --> CompIface
  PH --> EvalIface
  Runner --> CompIface
  Runner --> CmpIface
  HTTP -.->|implements| CompIface
  SimP -.->|implements| CompIface
  SimC -.->|implements| CompIface
  Runner -.->|implements| EvalIface
  ContentCmp -.->|implements| CmpIface
  CompIface --> DTOs
  CmpIface --> DTOs
```

| Principle | How it shows up |
|-----------|-----------------|
| **S**ingle responsibility | Handler = HTTP; Runner = async shadow; Comparator = diff; HTTPClient = remote calls |
| **O**pen/closed | New Completers (sim / HTTP / other) without changing handler |
| **L**iskov | Any `Completer` can be primary or candidate |
| **I**nterface segregation | Small `Completer`, `Evaluator`, `Comparator` interfaces |
| **D**ependency inversion | Handler depends on interfaces, not concrete LLM clients |

---

## 3. Shadow context survival

Gin cancels `c.Request.Context()` when the client disconnects. Shadow work must **not** use that cancellation.

```mermaid
sequenceDiagram
  participant C as Client
  participant H as PrimaryHandler
  participant P as Primary Completer
  participant S as Shadow Runner
  participant K as Candidate Completer

  C->>H: POST /v1/primary
  H->>P: Complete(reqCtx)
  P-->>H: primary response
  H-->>C: 200 + primary JSON
  Note over C: Client may close connection now

  H->>S: EvaluateAsync(reqCtx, req, primary)
  Note over S: ctx = WithoutCancel(reqCtx) + Timeout
  S->>K: Complete(detachedCtx)
  K-->>S: candidate response
  S->>S: Compare + log mismatch payloads
```

Key API: `context.WithoutCancel(reqCtx)` + `context.WithTimeout(...)`.

---

## 4. Component map (runtime)

```mermaid
flowchart TB
  subgraph app["shadow-llm-evaluator process"]
    R[Gin router]
    R -->|GET /healthz| Health[Health handler]
    R -->|POST /v1/primary| PH[PrimaryHandler]
    R -->|POST /simulate/primary| SimRoute[Simulator HTTP probe]

    PH --> Primary
    PH --> Shadow

    subgraph Primary["Primary path sync"]
      PComp[Completer]
    end

    subgraph Shadow["Shadow path async"]
      Runner[Runner]
      Cand[Completer]
      Cmp[Comparator]
      Runner --> Cand --> Cmp
    end
  end

  subgraph backends["Backends via env"]
    DO[DigitalOcean Inference<br/>inference.do-ai.run]
    LocalSim[In-process simulators]
  end

  PComp -->|PRIMARY_LLM_URL set| DO
  PComp -->|URL empty| LocalSim
  Cand -->|CANDIDATE_LLM_URL set| DO
  Cand -->|URL empty| LocalSim
```

---

## 5. Configuration & environments

```mermaid
flowchart LR
  APP_ENV[APP_ENV=local/dev/qa/prod] --> Loader[config.Load]
  Loader --> File["env/.env.&lt;APP_ENV&gt;"]
  File --> Cfg[Config struct]
  Cfg --> Main[Wire Completers + Runner + Gin]

  OS[OS / GitHub / App Platform env] -.->|overrides empty keys| Loader
```

| File | Typical use |
|------|-------------|
| `env/.env.local` | Simulators, no DO key |
| `env/.env.dev` | DO Inference + router |
| `env/.env.qa` | DO Inference (required key/models) |
| `env/.env.prod` | DO Inference (required key/models) |

Important env vars: `PRIMARY_LLM_URL`, `CANDIDATE_LLM_URL`, `PRIMARY_MODEL`, `CANDIDATE_MODEL`, `MODEL_ACCESS_KEY`.

---

## 6. CI/CD & deploy

```mermaid
flowchart LR
  Push[Push / PR to main] --> Quality[Job: quality<br/>gofmt, vet, golangci-lint, app spec]
  Push --> Test[Job: test<br/>go test -race, build]
  Quality --> Deploy
  Test --> Deploy
  Deploy[Job: deploy on main only] --> DO[DigitalOcean App Platform]
  DO --> Live[https://*.ondigitalocean.app]
```

Secrets (GitHub Actions):

- `DIGITALOCEAN_ACCESS_TOKEN` — App Platform deploy
- `MODEL_ACCESS_KEY` — Inference Bearer token injected into the app

---

## 7. Directory layout

```text
shadow-llm-evaluator/
├── main.go                 # wire config, completers, routes
├── Dockerfile
├── .do/app.yaml            # App Platform spec
├── .github/workflows/      # CI/CD
├── env/                    # .env.local / .dev / .qa / .prod
├── docs/
│   └── architecture.md     # this file
└── internal/
    ├── config/             # load env files → Config
    ├── handler/            # HTTP adapters
    ├── llm/                # Completer + HTTP client
    ├── shadow/             # async evaluator
    ├── compare/            # primary vs candidate diff
    ├── models/             # request/response DTOs
    └── simulator/          # local fake primary/candidate
```

---

## 8. Mismatch logging shape

When primary and candidate assistant contents differ, logs include clean JSON payloads:

```json
{
  "msg": "shadow evaluation mismatched",
  "primary_payload": {
    "model": "…",
    "content": "…",
    "extracted_json": {}
  },
  "candidate_payload": {
    "model": "…",
    "content": "…",
    "extracted_json": {}
  }
}
```
