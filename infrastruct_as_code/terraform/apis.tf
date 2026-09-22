locals {
  apis = [
    "run.googleapis.com",
    "firestore.googleapis.com",
    "artifactregistry.googleapis.com",
    "iam.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "billingbudgets.googleapis.com",

    # Firebase: identitytoolkit backs Firebase Auth (the admin login),
    # firebasehosting serves the CDN that sits in front of Cloud Run.
    # firebase.googleapis.com is what makes the GCP project visible in the
    # Firebase console at all.
    "firebase.googleapis.com",
    "firebasehosting.googleapis.com",
    "identitytoolkit.googleapis.com",

    # Image uploads from the admin UI land in a GCS bucket.
    "storage.googleapis.com",
  ]
}

resource "google_project_service" "apis" {
  for_each = toset(local.apis)
  project  = var.project_id
  service  = each.value

  # Leaving APIs enabled on destroy is deliberate. Disabling an API can
  # break unrelated resources that still depend on it, and re-enabling is
  # cheap — there is no cost to an enabled-but-unused API.
  disable_on_destroy = false
}
