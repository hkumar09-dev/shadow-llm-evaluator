# =============================================================================
# GitHub Actions secrets required for CI/CD
#
# Repo → Settings → Secrets and variables → Actions → New repository secret
#
# DIGITALOCEAN_ACCESS_TOKEN
#   Create at: https://cloud.digitalocean.com/account/api/tokens
#   Needs read/write access to App Platform.
#
# MODEL_ACCESS_KEY
#   Create at: DigitalOcean → Inference → Manage → Model Access Keys
#   Used at deploy time as the Bearer token for inference.do-ai.run
#
# Optional: create a GitHub Environment named "production" (used by deploy job)
#   Settings → Environments → New environment → production
# =============================================================================
