variable "db_url" {
  type    = string
  default = "postgres://postgres:postgres@localhost:5432/cosmosbdr?sslmode=disable"
}

env "local" {
  url = var.db_url
  
  // Tell Atlas to inspect your Go GORM models package
  src = "ent://src/db/models" // Note: For GORM, Atlas uses sqlschema or loads via schema loader, but the cleanest way is loading from the live DB schema after auto-migrating, OR using the Go schema loader.
  
  migration {
    dir = "file://src/db/migrations"
  }
}