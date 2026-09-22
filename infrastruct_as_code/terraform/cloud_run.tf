resource "google_cloud_run_v2_service" "server" {
  project  = var.project_id
  name     = "blog"
  location = var.region

  # The service is reached through Firebase Hosting's CDN, but it still has
  # to accept traffic from the public internet — Hosting proxies requests to
  # the *.run.app URL from Google's edge, not over a private network.
  ingress = "INGRESS_TRAFFIC_ALL"

  deletion_protection = false

  template {
    service_account = google_service_account.server.email

    scaling {
      # Zero is the whole cost model: no traffic, no instances, no charge.
      # The price is a cold start on the first request after idle, which the
      # CDN in front absorbs for all but a handful of cache misses.
      min_instance_count = 0
      max_instance_count = var.max_instances
    }

    containers {
      image = var.server_image

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }

        # Bill for CPU only while a request is in flight. The server does
        # nothing between requests, so paying for idle CPU would be pure
        # waste. The trade-off is that background goroutines are throttled
        # once a response is sent — anything that must finish has to finish
        # before the handler returns.
        cpu_idle = true
      }

      ports {
        container_port = 8080
      }

      env {
        name  = "GOOGLE_CLOUD_PROJECT"
        value = var.project_id
      }

      env {
        name  = "BLOG_BASE_URL"
        value = var.site_base_url
      }

      env {
        name  = "BLOG_UPLOADS_BUCKET"
        value = google_storage_bucket.uploads.name
      }

      startup_probe {
        tcp_socket {
          port = 8080
        }
        initial_delay_seconds = 0
        period_seconds        = 1
        failure_threshold     = 10
      }
    }

    # A page render is a Firestore read and a template execution. If that
    # has not finished in 15s something is wrong, and holding the request
    # open longer only burns billable time.
    timeout = "15s"

    # Go handles concurrent requests fine, and a higher number means fewer
    # instances for the same traffic — which is both cheaper and fewer cold
    # starts.
    max_instance_request_concurrency = 80
  }

  # The image is replaced by the deploy pipeline, not by Terraform. Without
  # this, every `terraform apply` after a deploy would try to roll the
  # service back to var.server_image.
  lifecycle {
    ignore_changes = [
      template[0].containers[0].image,
      client,
      client_version,
    ]
  }

  depends_on = [google_project_service.apis]
}

# The blog is public. Firebase Hosting forwards end-user requests without
# credentials, so the service has to allow unauthenticated invocations —
# authentication for the admin API happens inside the app, by verifying a
# Firebase ID token and checking the UID against an allowlist.
resource "google_cloud_run_v2_service_iam_member" "public" {
  project  = var.project_id
  location = google_cloud_run_v2_service.server.location
  name     = google_cloud_run_v2_service.server.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
