variable "project_id" {
  description = "GCP project ID"
  type        = string
  default     = "cindle-blog"
}

variable "region" {
  description = "GCP region for all resources. Also the Firestore location, which is permanent once the database exists."
  type        = string
  default     = "asia-east1"
}

variable "billing_account" {
  description = "Billing account ID for the budget alert. Format: XXXXXX-XXXXXX-XXXXXX. Find with: gcloud billing accounts list"
  type        = string
}

variable "budget_amount" {
  description = "Monthly budget alert threshold, in the billing account's own currency (TWD for this account). The blog is designed to stay inside the free tier, so any non-zero spend is a signal that something is wrong — hence a deliberately tiny number."
  type        = number
  default     = 1
}

variable "server_image" {
  description = "Container image for cmd/server. Defaults to a public placeholder so the first apply can stand the service up before any real image exists."
  type        = string
  default     = "us-docker.pkg.dev/cloudrun/container/hello"
}

variable "max_instances" {
  description = "Hard cap on Cloud Run instances. This is a cost guard, not a capacity plan: a personal blog never needs to scale, and an unbounded max is the most plausible way this project stops being free (a crawler storm or a bug turns into real money)."
  type        = number
  default     = 3
}

variable "site_base_url" {
  description = "Public origin of the site, no trailing slash. Feeds view.Site.BaseURL, which every canonical URL, og:image and sitemap entry is built from. Points at the Cloud Run URL until the custom domain is live."
  type        = string
  default     = ""
}

variable "admin_uids" {
  description = "Firebase Auth UIDs allowed to use the admin API. Get yours from the Firebase console under Authentication → Users after signing in once. Not a secret — a UID identifies an account but grants nothing on its own; the server still verifies a signed ID token before comparing against this list. Empty means the admin API is closed to everyone, which is the correct default: a typo here should lock you out, never let the world in."
  type        = list(string)
  default     = []
}
