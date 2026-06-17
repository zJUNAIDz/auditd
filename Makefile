-include .env
export

.PHONY: migrate-up migrate-down sqlc

compose-up:
	docker compose -f compose.dev.yaml up -d
compose-down:
	docker compose -f compose.dev.yaml down

migrate-up:
	migrate \
	-path db/migrations \
	-database "$(DB_URL)" \
	up

migrate-down:
	migrate \
	-path db/migrations \
	-database "$(DB_URL)" \
	down 1

sqlc:
	sqlc generate