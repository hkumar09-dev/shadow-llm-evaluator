# shadow-llm-evaluator

Go service that serves a synchronous primary LLM response and asynchronously shadow-evaluates a candidate model.

## Run locally

```bash
APP_ENV=local go run .
curl -X POST http://127.0.0.1:8080/v1/primary \
  -H 'Content-Type: application/json' \
  -d '{"messages":[{"role":"user","content":"hello"}]}'
```

Env files live in `env/` (`.env.local`, `.env.dev`, `.env.qa`, `.env.prod`).

## CI/CD (GitHub Actions)

Workflow: [`.github/workflows/ci-cd.yml`](.github/workflows/ci-cd.yml)

| Job | When | What |
|-----|------|------|
| **quality** | PR + push | `gofmt`, `go vet`, `golangci-lint`, validate `.do/app.yaml` |
| **test** | PR + push | `go test -race`, coverage, `go build` |
| **deploy** | push to `main` only | DigitalOcean App Platform via `digitalocean/app_action/deploy@v2` |

### Required GitHub secrets

Repo → **Settings → Secrets and variables → Actions**:

| Secret | Where to get it |
|--------|-----------------|
| `DIGITALOCEAN_ACCESS_TOKEN` | [API tokens](https://cloud.digitalocean.com/account/api/tokens) (Apps read/write) |
| `MODEL_ACCESS_KEY` | Inference → Manage → Model Access Keys |

Also create a GitHub **Environment** named `production` (Settings → Environments) used by the deploy job.

See [`.github/SECRETS.md`](.github/SECRETS.md).

## Deploy to DigitalOcean App Platform

Deploys happen automatically on merge/push to `main` after CI passes.

Manual first-time create (optional):

```bash
doctl auth init
doctl apps create --spec .do/app.yaml
```

Verify:

```bash
curl https://<your-app>.ondigitalocean.app/healthz
curl -X POST https://<your-app>.ondigitalocean.app/v1/primary \
  -H 'Content-Type: application/json' \
  -d '{"messages":[{"role":"user","content":"hello from digitalocean"}]}'
```
