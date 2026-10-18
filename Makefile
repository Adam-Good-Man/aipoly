.PHONY: build-api run-api build-client dev-client dev

build-api:
	cd api && env GOSUMDB=sum.golang.org GOTOOLCHAIN=auto go build -o bin/aipoly .

run-api:
	cd api && export $$(grep -v '^#' ../.env | grep -v '^$$' | xargs) && ./bin/aipoly

build-client:
	cd client && npm run build

dev-client:
	cd client && npm run dev

dev: build-api
	@echo "Starting API on :8081 and client on :5173 ..."
	@(cd api && export $$(grep -v '^#' ../.env | grep -v '^$$' | xargs) && ./bin/aipoly) & \
	 cd client && npm run dev
