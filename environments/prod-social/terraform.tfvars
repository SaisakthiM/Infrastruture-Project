# ─── SOCIAL MEDIA (Kubernetes) ────────────────────────────────
social_db_password    = "password"
social_minio_user     = "minio"
social_minio_password = "minio123"

load_images = false

# Set this once, then `sed -i 's#git@github.com:SaisakthiM/Coding-Project.git#git@github.com:you/your-repo.git#'
# gitops/social-media/apps/social-workload-app.yaml gitops/observability/apps/*.yaml`
# to update the rest -- those are plain YAML, not Terraform, so this variable
# can't reach into them.
gitops_repo_url = "git@github.com:SaisakthiM/Coding-Project.git"
gitops_repo_ssh_key = <<EOF
-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACB8j5sJmIoastKDs9642Q8VgjiFOKcmE/HTGz+ttqiMTAAAAJiAfODpgHzg
6QAAAAtzc2gtZWQyNTUxOQAAACB8j5sJmIoastKDs9642Q8VgjiFOKcmE/HTGz+ttqiMTA
AAAEDrEj7DqLq+Pv3PNOvR65Cvb3X7VTXR5APY6wMiR1Ir73yPmwmYihqy0oOz3rjZDxWC
OIU4pyYT8dMbP622qIxMAAAAEWFyZ29jZC1kZXBsb3kta2V5AQIDBA==
-----END OPENSSH PRIVATE KEY-----
EOF
