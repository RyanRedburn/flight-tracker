MIGRATE ?= migrate
MIGRATIONS_PATH ?= /migrations/sqlite
DATABASE_URL ?= sqlite3:///data/flight-tracker.db
SQLITE_DB ?= /data/flight-tracker.db
FLIGHT_DATA_CSV ?= /test-data/On_Time_Marketing_Carrier_On_Time_Performance_(Beginning_January_2018)_2026_4.csv

.PHONY: up down version force shell seed-flights

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

seed-flights:
	@test -f "$(FLIGHT_DATA_CSV)" || (echo "missing $(FLIGHT_DATA_CSV)" && exit 1)
	@count=$$(sqlite3 $(SQLITE_DB) "SELECT COUNT(*) FROM on_time_flights;"); \
	if [ "$$count" -gt 0 ]; then \
	  echo "on_time_flights already has $$count rows, skipping import"; \
	else \
	  printf '.mode csv\n.import --skip 1 %s on_time_flights\n' '$(FLIGHT_DATA_CSV)' | sqlite3 $(SQLITE_DB); \
	fi
