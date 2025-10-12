.PHONY: help build build-backend build-backend-dev build-dist clean-dist build-lpk dev-backend dev-frontend frontend-install frontend-build frontend-test frontend-clean build-local test test-coverage fmt vet lint tidy check audit clean push deploy-prod deploy-dev-full deploy-backend-dev deploy-frontend deploy-lpk deploy all trivy-server-start trivy-server-stop trivy-server-status trivy-server-logs

# Default registry and image names
LAZYCAT_BOX_NAME ?= mybox
REGISTRY ?= docker-registry-ui.${LAZYCAT_BOX_NAME}.heiyu.space
BACKEND_IMAGE ?= $(REGISTRY)/trivy/backend
VERSION ?= latest
PLATFORM ?= linux/amd64
GOOS ?= linux
GOARCH ?= amd64

# Trivy environment variables
TRIVY_SERVER ?= http://trivy-server:4954
TRIVY_SERVER_LOCAL ?= http://127.0.0.1:4954
DEFAULT_REGISTRY ?= docker.io/
CONFIG_DIR ?= /configs
CONFIG_DIR_LOCAL ?= ./configs

HELP_FUN = \
	%help; while(<>){push@{$$help{$$2//'options'}},[$$1,$$3] \
	if/^([\w-_]+)\s*:.*\#\#(?:@(\w+))?\s(.*)$$/}; \
	print"\033[1m$$_:\033[0m\n", map"  \033[36m$$_->[0]\033[0m".(" "x(20-length($$_->[0])))."$$_->[1]\n",\
	@{$$help{$$_}},"\n" for keys %help; \

help: ##@General Show this help
	@echo -e "Usage: make \033[36m<target>\033[0m\n"
	@perl -e '$(HELP_FUN)' $(MAKEFILE_LIST)

trivy-server-start: ##@TrivyServer Start Trivy Server container
	@echo "Starting Trivy Server..."
	@if docker ps -a --format '{{.Names}}' | grep -q '^trivy-server$$'; then \
		if docker ps --format '{{.Names}}' | grep -q '^trivy-server$$'; then \
			echo "Trivy Server is already running"; \
		else \
			echo "Starting existing Trivy Server container..."; \
			docker start trivy-server; \
		fi \
	else \
		echo "Creating and starting new Trivy Server container..."; \
		docker run -d --name trivy-server \
			-p 4954:4954 \
			aquasec/trivy:latest \
			server --listen 0.0.0.0:4954; \
	fi
	@echo "Trivy Server is running at $(TRIVY_SERVER_LOCAL)"

trivy-server-stop: ##@TrivyServer Stop Trivy Server container
	@echo "Stopping Trivy Server..."
	@docker stop trivy-server 2>/dev/null || echo "Trivy Server is not running"

trivy-server-status: ##@TrivyServer Show Trivy Server status
	@echo "Trivy Server status:"
	@docker ps -a --filter "name=trivy-server" --format "Name: {{.Names}}\nStatus: {{.Status}}\nPorts: {{.Ports}}\nImage: {{.Image}}" || echo "Trivy Server container not found"

trivy-server-logs: ##@TrivyServer Show Trivy Server logs
	@docker logs -f trivy-server

dev-backend: trivy-server-start ##@Development Run backend locally for development
	cd backend && \
	export TRIVY_TRIVY_SERVER="$(TRIVY_SERVER_LOCAL)" && \
	export TRIVY_DEFAULT_REGISTRY="$(DEFAULT_REGISTRY)" && \
	export TRIVY_CONFIG_DIR="$(CONFIG_DIR_LOCAL)" && \
	export TRIVY_ALLOW_PASSWORD_SAVE="true" && \
	go run cmd/server/main.go

dev-frontend: ##@Development Run frontend locally for development
	cd frontend && npm start

frontend-install: ##@Frontend Install frontend dependencies
	@echo "Installing frontend dependencies..."
	cd frontend && npm install
	@echo "Frontend dependencies installed!"

frontend-build: build-dist ##@Frontend Build frontend (alias for build-dist)

frontend-test: ##@Frontend Run frontend tests
	@echo "Running frontend tests..."
	cd frontend && npm test
	@echo "Frontend tests completed!"

frontend-clean: ##@Frontend Clean frontend build artifacts
	@echo "Cleaning frontend build artifacts..."
	-rm -rf ./frontend/build
	-rm -rf ./frontend/node_modules/.cache
	@echo "Frontend clean completed!"

frontend-outdated: ##@Frontend Check outdated frontend dependencies
	@echo "Checking outdated frontend dependencies..."
	@CURRENT_REGISTRY=$$(npm config get registry); \
	npm config set registry https://registry.npmjs.org/; \
	cd frontend && npm outdated; \
	npm config set registry $$CURRENT_REGISTRY || true
	@echo "Check completed! (exit code 1 if outdated packages exist)"

frontend-update: ##@Frontend Update frontend dependencies to compatible versions
	@echo "Updating frontend dependencies to compatible versions..."
	@CURRENT_REGISTRY=$$(npm config get registry); \
	npm config set registry https://registry.npmjs.org/; \
	cd frontend && npm update; \
	npm config set registry $$CURRENT_REGISTRY
	@echo "Update completed!"

frontend-upgrade: ##@Frontend Upgrade all frontend dependencies to latest versions (requires npm-check-updates)
	@echo "Upgrading all frontend dependencies to latest versions..."
	@if ! command -v ncu >/dev/null 2>&1; then \
		echo "Error: npm-check-updates is not installed"; \
		echo "Install it with: npm install -g npm-check-updates"; \
		exit 1; \
	fi
	@CURRENT_REGISTRY=$$(npm config get registry); \
	npm config set registry https://registry.npmjs.org/; \
	cd frontend && ncu -u && npm install; \
	npm config set registry $$CURRENT_REGISTRY
	@echo "Upgrade completed!"

frontend-upgrade-interactive: ##@Frontend Interactively upgrade frontend dependencies (requires npm-check-updates)
	@echo "Interactively upgrading frontend dependencies..."
	@if ! command -v ncu >/dev/null 2>&1; then \
		echo "Error: npm-check-updates is not installed"; \
		echo "Install it with: npm install -g npm-check-updates"; \
		exit 1; \
	fi
	@CURRENT_REGISTRY=$$(npm config get registry); \
	npm config set registry https://registry.npmjs.org/; \
	cd frontend && ncu -i && npm install; \
	npm config set registry $$CURRENT_REGISTRY
	@echo "Interactive upgrade completed!"

test: ##@Development Run backend tests
	cd backend && go test -v ./...

test-coverage: ##@Development Run backend tests with coverage
	cd backend && go test -v -coverprofile=coverage.out ./...
	cd backend && go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: backend/coverage.html"

fmt: ##@Development Format Go code
	cd backend && go fmt ./...

vet: ##@Development Run go vet
	cd backend && go vet ./...

lint: fmt vet ##@Development Run fmt and vet

tidy: ##@Development Run go mod tidy
	cd backend && go mod tidy

check: lint test ##@Development Run lint and tests

audit: ##@Development Scan frontend for security vulnerabilities
	@echo "Scanning frontend dependencies for vulnerabilities..."
	@CURRENT_REGISTRY=$$(npm config get registry); \
	npm config set registry https://registry.npmjs.org/; \
	cd frontend && npm audit; \
	AUDIT_EXIT=$$?; \
	npm config set registry $$CURRENT_REGISTRY; \
	exit $$AUDIT_EXIT

build: build-backend build-dist ##@Build Build backend image and frontend dist

build-backend: ##@Build Build backend Docker image (production)
	@echo "Building backend image: $(BACKEND_IMAGE):$(VERSION) for platform $(PLATFORM)"
	cd backend && docker build --platform $(PLATFORM) --target prod -t $(BACKEND_IMAGE):$(VERSION) .
	@echo "Backend image built successfully!"

build-backend-dev: build-local ##@Build Build backend Docker image (development with local binary)
	@echo "Building backend dev image: $(BACKEND_IMAGE):${VERSION} for platform $(PLATFORM)"
	cd backend && docker build --platform $(PLATFORM) --target dev -t $(BACKEND_IMAGE):${VERSION} .
	@echo "Backend dev image built successfully!"

build-dist: ##@Build Build frontend into dist directory
	@echo "Building frontend to dist directory..."
	sh build.sh
	@echo "Frontend dist built successfully!"

build-lpk: ##@Build Build LPK package (requires lzc-cli)
	@echo "Building LPK package..."
	lzc-cli project build
	@echo "LPK package built successfully!"

build-local: ##@Build Build backend binary locally
	cd backend && CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o trivy-web-server cmd/server/main.go

clean: ##@Maintenance Clean dist directory and backend binary
	@echo "Cleaning dist directory..."
	-rm -rf ./dist
	@echo "Cleaning backend binary..."
	-cd backend && rm -f trivy-web-server
	@echo "Clean completed!"

clean-dist: ##@Maintenance Clean dist directory only
	@echo "Cleaning dist directory..."
	-rm -rf ./dist

push: ##@Maintenance Push backend image to registry
	@echo "Pushing backend image..."
	docker push $(BACKEND_IMAGE):$(VERSION)
	@echo "Backend image pushed successfully!"

deploy-prod: build-backend push build-dist deploy-lpk ##@Deploy Production deployment (backend prod + frontend + lpk)

deploy-dev-full: build-backend-dev push build-dist deploy-lpk ##@Deploy Development full deployment (backend dev + frontend + lpk)

deploy-backend-dev: build-backend-dev push deploy-lpk ##@Deploy Deploy backend dev only (backend dev + lpk)

deploy-frontend: build-dist deploy-lpk ##@Deploy Deploy frontend only (frontend + lpk)

deploy-lpk: build-lpk ##@Deploy Deploy LPK only (no rebuild frontend/backend)
	@echo "Deploying LPK package..."
	@LPK_FILE=$$(ls -t *-v*.lpk 2>/dev/null | head -n 1); \
	if [ -z "$$LPK_FILE" ]; then \
		echo "Error: No LPK file found"; \
		exit 1; \
	fi; \
	echo "Installing $$LPK_FILE..."; \
	lzc-cli app install "$$LPK_FILE"
	@echo "Deployment completed!"

deploy: deploy-prod ##@Aliases Alias for deploy-prod (backward compatibility)

all: deploy-prod ##@Aliases Alias for deploy-prod (default target)
