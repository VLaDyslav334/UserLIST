.PHONY: up down logs

up:
	@ docker compose up -d

down:
	@ docker compose down

logs:
	@ docker compose logs -f app

restart:
	@ docker compose restart app

# Проверка конфигурации
check-env:
	@test -n "$(HOST)" || (echo "Error: HOST is not set" && exit 1)
	@test -n "$(POSTGRES_USER)" || (echo "Error: POSTGRES_USER is not set" && exit 1)
	@test -n "$(POSTGRES_PASSWORD)" || (echo "Error: POSTGRES_PASSWORD is not set" && exit 1)
	@test -n "$(POSTGRES_DB)" || (echo "Error: POSTGRES_DB is not set" && exit 1)
	@echo "All environment variables are set correctly"

deploy: check-env up