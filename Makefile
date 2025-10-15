build:
	docker compose build
run:
	docker compose up

test:
	go test -v ./...

migrate:
	migrate -path ./schema -database 'postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@db:5432/${POSTGRES_DB}?sslmode=disable' up

swag:
	swag init -g cmd/main.go