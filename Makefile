.PHONY: dev test build lint typecheck docker-up docker-down

dev:
	npm run dev

typecheck:
	npm run typecheck

test:
	npx vitest run

build:
	npm run build

lint:
	npm run lint

docker-up:
	docker-compose up -d --build

docker-down:
	docker-compose down
