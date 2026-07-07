MIGRATE ?= migrate
MIGRATIONS_PATH ?= /migrations/sqlite
DATABASE_URL ?= sqlite3:///data/flight-tracker.db
SQLITE_DB ?= /data/flight-tracker.db

.PHONY: up down version force shell

up:
	$(MIGRATE) -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" up

down:
	$(MIGRATE) -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" down 1

version:
	$(MIGRATE) -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" version

# Run with VERSION set, e.g. make force VERSION=1
force:
	@test -n "$(VERSION)" || (echo "VERSION is required, e.g. make force VERSION=1" && exit 1)
	$(MIGRATE) -path $(MIGRATIONS_PATH) -database "$(DATABASE_URL)" force $(VERSION)

shell:
	sqlite3 $(SQLITE_DB)
