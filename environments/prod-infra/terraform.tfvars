# ─── n8n ────────────────────────────────────────────────────
n8n_env = {
  N8N_PORT                        = "5678"
  N8N_HOST                        = "0.0.0.0"
  N8N_PROTOCOL                    = "https"
  N8N_PATH                        = "/n8n/"
  N8N_PROXY_HOPS                  = "1"
  N8N_EDITOR_BASE_URL             = "https://saisakthi.qzz.io/n8n"
  WEBHOOK_URL                     = "https://saisakthi.qzz.io/n8n"
  N8N_BASIC_AUTH_ACTIVE           = "true"
  N8N_BASIC_AUTH_USER             = "admin"
  N8N_BASIC_AUTH_PASSWORD         = "saisakthi2008"
  GENERIC_TIMEZONE                = "Asia/Kolkata"
  TZ                              = "Asia/Kolkata"
  N8N_METRICS                     = "true"
  N8N_RUNNERS_DISABLED            = "true"
  N8N_ENFORCE_SETTINGS_PERMISSION = "true"
  N8N_DIAGNOSTICS_ENABLED         = "true"
  N8N_TELEMETRY_ENABLED           = "true"
  N8N_AI_ASSISTANT_BASE_URL       = "http://ollama:11434"
  N8N_DISABLE_PUSH_ORIGIN_CHECK   = "true" # was an unquoted `true` bool in the original tfvars -- fixed, this var is map(string)

  N8N_CONTENT_SECURITY_POLICY = <<JSON
{
  "default-src": ["'self'", "blob:", "data:"],
  "script-src": ["'self'", "'unsafe-inline'", "'unsafe-eval'", "blob:", "data:", "https://static.cloudflareinsights.com"],
  "style-src": ["'self'", "'unsafe-inline'", "https://fonts.googleapis.com"],
  "font-src": ["'self'", "https://fonts.gstatic.com", "data:"],
  "img-src": ["'self'", "data:", "blob:", "https:", "http:"],
  "connect-src": ["'self'", "ws:", "wss:", "https:", "http:"],
  "worker-src": ["'self'", "blob:"],
  "frame-ancestors": ["http://n8n", "https://saisakthi.qzz.io"]
}
JSON
}

# ─── Observability ──────────────────────────────────────────
observability_redis_password = "redis-password" # TODO: rotate

gitops_repo_url = "git@github.com:SaisakthiM/Coding-Project.git"

atlantis_gh_user = "SaisakthiM"
atlantis_gh_token = "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"
atlantis_gh_webhook_secret = "saisakthi@2008"
