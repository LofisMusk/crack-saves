.PHONY: server-test server-build server-run docker-up docker-down client-dev client-build

server-test:
	cd server && go test ./...

server-build:
	cd server && go build -o cracked-saves-server .

server-run:
	cd server && go run .

docker-up:
	cd docker && docker compose up -d --build

docker-down:
	cd docker && docker compose down

client-dev:
	cd client && npm install && npm run tauri dev

client-build:
	cd client && npm install && npm run tauri build
