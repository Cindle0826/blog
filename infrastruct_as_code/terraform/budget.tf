# Cloud Billing 的 Budget API 會把 projects/<id> 正規化成 projects/<number>
# 存回去，所以設定裡如果寫 project_id，每次 plan 都會看到一個改不掉的差異
# （想把 number 改回 id，改完 API 又轉成 number）。查一次編號來用就穩定了。
data "google_project" "this" {
  project_id = var.project_id
}

resource "google_billing_budget" "monthly" {
  billing_account = var.billing_account
  display_name    = "cindle-blog monthly budget"

  budget_filter {
    projects = ["projects/${data.google_project.this.number}"]
  }

  amount {
    specified_amount {
      currency_code = "TWD"
      units         = tostring(var.budget_amount)
    }
  }

  # threshold_percent is a fraction of var.budget_amount, so with a NT$1
  # budget these land at roughly NT$0.5 / NT$1 / NT$10 actual spend.
  #
  # A budget alert only sends email — GCP has no built-in "shut everything
  # down at $X" switch. The actual spend guard is max_instances in
  # cloud_run.tf; this is the tripwire that tells you to go look.
  threshold_rules {
    threshold_percent = 0.5
  }

  threshold_rules {
    threshold_percent = 1.0
  }

  threshold_rules {
    threshold_percent = 10.0
  }

  depends_on = [google_project_service.apis]
}
