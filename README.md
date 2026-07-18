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

## Deploy to DigitalOcean App Platform

### 1. Prerequisites
- DigitalOcean account
- GitHub repo connected to DigitalOcean (App Platform → GitHub)
- [`doctl`](https://docs.digitalocean.com/reference/doctl/how-to/install/) installed

### 2. Authenticate

```bash
doctl auth init
# paste your DigitalOcean API token (read/write)
```

### 3. Create the app from the repo spec

```bash
# from repo root (after pushing this code to GitHub main)
doctl apps create --spec .do/app.yaml
```

List apps and get the live URL:

```bash
doctl apps list
doctl apps get <APP_ID> --format DefaultIngress,ID,Spec.Name
```

### 4. Verify

```bash
curl https://<your-app>.ondigitalocean.app/healthz
curl -X POST https://<your-app>.ondigitalocean.app/v1/primary \
  -H 'Content-Type: application/json' \
  -d '{"messages":[{"role":"user","content":"hello from digitalocean"}]}'
```

### 5. Point at real LLMs (optional)

In the DigitalOcean dashboard → App → Settings → App-Level Environment Variables (or edit `.do/app.yaml` and `doctl apps update`):

- `APP_ENV=prod`
- `PRIMARY_LLM_URL=https://...`
- `CANDIDATE_LLM_URL=https://...`

Then redeploy.

### UI alternative

1. DigitalOcean → **Apps** → **Create App**
2. Choose GitHub repo `hkumar09-dev/shadow-llm-evaluator` (branch `main`)
3. Detect Dockerfile / or upload `.do/app.yaml`
4. Set HTTP port `8080`, health check `/healthz`
5. Deploy
