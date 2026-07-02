DB_URL=postgres://postgres:postgres@localhost:5432/expensetracker?sslmode=disable
MIGRATIONS_PATH=./migrations
GOCACHE_DIR=/tmp/go-build-cache
COVERAGE_PROFILE=/tmp/code.cover.out

.PHONY: migrate-create
migrate-create:
	migrate create -ext sql -dir $(MIGRATIONS_PATH) -seq $(name)

.PHONY: migrate-up
migrate-up:
	migrate -path $(MIGRATIONS_PATH) -database "$(DB_URL)" up

.PHONY: migrate-down
migrate-down:
	migrate -path $(MIGRATIONS_PATH) -database "$(DB_URL)" down 1

.PHONY: migrate-reset
migrate-reset:
	migrate -path $(MIGRATIONS_PATH) -database "$(DB_URL)" down -all
	migrate -path $(MIGRATIONS_PATH) -database "$(DB_URL)" up

.PHONY: migrate-force
migrate-force:
	migrate -path $(MIGRATIONS_PATH) -database "$(DB_URL)" force $(version)

.PHONY: check
check:
	find . -type f -name '*.go' \
		-not -path './.git/*' \
		-not -path './tmp/*' \
		-exec gofmt -w {} +
	env GOCACHE=$(GOCACHE_DIR) go vet ./...
	env GOCACHE=$(GOCACHE_DIR) go test ./... -coverprofile=$(COVERAGE_PROFILE)
	env GOCACHE=$(GOCACHE_DIR) go tool cover -func=$(COVERAGE_PROFILE) | tail -n 1
