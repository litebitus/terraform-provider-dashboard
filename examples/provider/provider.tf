terraform {
  required_providers {
    dashboard = {
      source = "litebitus/dashboard"
    }
  }
}

provider "dashboard" {
  default_target_schemas = {
    grafana = "grafana-v2-13"
  }
}
