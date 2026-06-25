# ─── SOCIAL MEDIA ─────────────────────────────────────────────
# social_db_name / social_db_user were dropped as variables -- they're
# non-secret identifiers, now hardcoded literally in
# gitops/social-media/raw/03-postgres-statefulset.yaml and
# gitops/social-media/raw/05-backend-deployment.yaml ("socialdb" / "admin").
variable "social_db_password" { sensitive = true }
variable "social_minio_user"  {}
variable "social_minio_password" { sensitive = true }

variable "load_images" {
  type    = bool
  default = false
}

variable "gitops_repo_url" {
  description = "Git URL ArgoCD pulls gitops/ from. Must match the repo you push this whole infra/ tree to."
  type        = string
  default     = "https://github.com/SaisakthiM/Coding-Project"
}

variable "gitops_repo_ssh_key" {
  type      = string
  sensitive = true
}
