resource "google_artifact_registry_repository" "images" {
  project       = var.project_id
  location      = var.region
  repository_id = "blog"
  format        = "DOCKER"

  # Artifact Registry's free tier is only 0.5 GiB/month. A distroless Go
  # image is ~18MB, so that is roughly 28 images — which sounds like plenty
  # until you notice every deploy pushes one and nothing ever deletes them.
  # Without a cleanup policy this is the most likely first line on a bill
  # that was supposed to be zero.
  cleanup_policies {
    id     = "keep-recent"
    action = "KEEP"
    most_recent_versions {
      keep_count = 5
    }
  }

  cleanup_policies {
    id     = "delete-old"
    action = "DELETE"
    condition {
      tag_state  = "ANY"
      older_than = "2592000s" # 30 days
    }
  }

  depends_on = [google_project_service.apis]
}
