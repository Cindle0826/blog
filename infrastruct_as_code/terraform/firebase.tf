// Firebase Web App registration for the admin SPA.
//
// This is what the browser needs before it can start a Google sign-in: a set
// of identifiers telling the Firebase SDK which project to talk to. Clicking
// through the Firebase console would produce the same thing, but then the
// values live only in a screenshot someone has to copy by hand. Here they are
// terraform outputs, and re-creating the project reproduces them.
//
// Contrast with the Google sign-in provider itself, which is deliberately NOT
// managed here — see README. That one needs an OAuth client_id/client_secret,
// and GCP has no API for creating OAuth clients, so Terraform would add
// manual steps rather than remove them. This resource needs no secrets.
//
// Requires the google-beta provider; the Firebase resources have never
// graduated to the GA provider.
resource "google_firebase_web_app" "admin" {
  provider     = google-beta
  project      = var.project_id
  display_name = "blog-admin"

  # Firebase provisions a browser API key automatically when one is not
  # supplied. That key is not a credential — see the output descriptions.
  deletion_policy = "DELETE"

  depends_on = [google_project_service.apis]
}

data "google_firebase_web_app_config" "admin" {
  provider   = google-beta
  project    = var.project_id
  web_app_id = google_firebase_web_app.admin.app_id
}
