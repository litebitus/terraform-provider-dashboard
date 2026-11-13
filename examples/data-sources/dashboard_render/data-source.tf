data "dashboard_render" "mock_service" {
  spec = file("${path.module}/../../dashboards/mock-service.yml")
}

data "dashboard_render" "mock_slots" {
  spec = file("${path.module}/../../dashboards/mock-game.yml")
  inputs = {
    game = "mock-slots"
    env  = "production"
  }
}

resource "aws_s3_object" "mock_service_dashboard" {
  bucket       = "mock-config-bucket"
  key          = "grafana/dashboards/mock-owner/mock-service.json"
  content_type = "application/json"
  content      = data.dashboard_render.mock_service.json
}
