up:
	docker compose up -d

up_build:
	docker compose up -d --build

down:
	docker compose down --remove-orphans --volumes

log1:
	docker compose logs -f webserver1

log2:
	docker compose logs -f webserver2