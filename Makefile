-include .env
export

.PHONY: migrate-up migrate-down sqlc

migrate-up:
	migrate \
	-path internal/db/migrations \
	-database "$(DB_URL)" \
	up

migrate-down:
	migrate \
	-path internal/db/migrations \
	-database "$(DB_URL)" \
	down 1

sqlc:
	sqlc generate