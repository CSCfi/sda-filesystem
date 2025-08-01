.PHONY: help all remote local cli gui gui_build gui_prod wails_update requirements clean down get_env run_profiles build_profiles exec envs _wait_for_container _follow_logs

MAKEFLAGS += --no-print-directory

PROFILES := fuse krakend keystone
IS_UBUNTU_24_04 := $(if $(filter Ubuntu,$(shell lsb_release -si 2>/dev/null)), $(if $(filter 24.04,$(shell lsb_release -sr 2>/dev/null)),true,false),false)
LOG ?= info

WAILS_FLAGS =
ifeq ($(IS_UBUNTU_24_04),true)
	WAILS_FLAGS += -tags webkit2_41
endif
WAILS_FLAGS += $(shell command -v upx >/dev/null && echo -upx)

profile_args = $(foreach a,$1,--profile $a --env-file .env.$(firstword $(subst -, ,$a)))

define write_secret
printf "%s=" $(1) >> .env; \
vault kv get --field=$(3) secret/$(2) >> .env; \
echo >> .env;
endef

define docker_cmd
cd dev-tools/compose/; docker compose --env-file .env $(call profile_args, $1) $(2)
endef

define docker_up_aftermath
$(MAKE) _follow_logs
$(MAKE) _wait_for_container CONTAINER_NAME=data-upload
if [ "${UNAME}" = "Darwin" ]; then \
	osascript \
		-e 'tell application "Terminal"' \
			-e 'activate' \
			-e 'tell application "System Events" to keystroke tab using {control down, shift down}' \
		-e 'end tell'; \
fi
endef

# If the first argument is "run_profiles" or "build_profiles"
ifneq ($(filter $(firstword $(MAKECMDGOALS)),run_profiles build_profiles),)
  # use the rest as arguments for "run_profiles" or "build_profiles"
  # they need to be in an specific order, hence the filtering
  RUN_ARGS := $(filter $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS)),$(PROFILES))
  # ...and turn them into do-nothing targets
  $(eval $(RUN_ARGS):;@:)
endif

# Default target
# Print list of available targets. Only rows in the format `target: ## description` are printed
help:
	@echo "Available targets:"
	@awk '/^[a-zA-Z0-9_-]+:.*?## / {printf "  %-20s %s\n", $$1, substr($$0, index($$0, "##") + 3)}' $(MAKEFILE_LIST)

all: down ## Run 'make local gui'
	$(MAKE) local gui

requirements: ## Install dependencies and create .env file with vault secrets
	cp dev-tools/.env.example dev-tools/compose/.env
	@$(MAKE) get_env
	set -a; source dev-tools/compose/.env; docker login $${ARTIFACTORY_SERVER}
	pnpm install --prefix frontend
	pnpm --prefix frontend run build

remote: down ## Only set up mock terminal-proxy and connect to test cluster KrakenD
	$(call docker_cmd,,up -d --build)
	@$(call docker_up_aftermath)

local: down ## Run components locally
	$(call docker_cmd,krakend keystone,build)
	$(call docker_cmd,keystone-creds,up -d)
	@$(MAKE) _wait_for_container CONTAINER_NAME=findata-creds
	$(call docker_cmd,krakend keystone,up -d)
	@$(call docker_up_aftermath)

cli: ## Run CLI version of filesystem on your own computer
	@$(MAKE) _wait_for_container CONTAINER_NAME=data-upload
	@export $$(grep -E 'PROXY_URL|SDS_ACCESS_TOKEN|CONFIG_ENDPOINT' dev-tools/compose/.env | xargs); \
	trap 'exit 0' INT; go run ./cmd/cli -loglevel=$(LOG) import

gui: wails_update ## Run GUI version of filesystem on your own computer
	@$(MAKE) _wait_for_container CONTAINER_NAME=data-upload
	@export $$(grep -E 'PROXY_URL|SDS_ACCESS_TOKEN|CONFIG_ENDPOINT' dev-tools/compose/.env | xargs); \
	trap 'exit 0' INT; cd cmd/gui; \
	if [ $(IS_UBUNTU_24_04) = true ]; then \
		wails dev -race -tags webkit2_41; \
	else \
		wails dev -race; \
	fi

gui_build: wails_update ## Compile a production-ready GUI binary and save it in build/bin
	cd cmd/gui; wails build $(WAILS_FLAGS) -trimpath -clean -s

gui_prod: ## Build and run a production-ready GUI binary
	@$(MAKE) gui_build
	@$(MAKE) _wait_for_container CONTAINER_NAME=data-upload
	@export $$(grep -E 'PROXY_URL|SDS_ACCESS_TOKEN|CONFIG_ENDPOINT' dev-tools/compose/.env | xargs); \
	./build/bin/data-gateway

wails_update: ## Update Wails version to match go.mod
	@wails_cli_version=$$(wails version | head -n 1); \
	go_mod_version=$$(grep -w 'github.com/wailsapp/wails/v2' go.mod | awk '{print $$2}'); \
	if [ "$$wails_cli_version" != "$$go_mod_version" ]; then \
		echo "❗ Wails version does not match go.mod. Updating Wails..."; \
		go install github.com/wailsapp/wails/v2/cmd/wails@$${go_mod_version}; \
	fi

clean: down ## Stop running containers, delete volumes, and remove vault secrets from .env
	@cd dev-tools/compose/; sed -i.bak '/### VAULT SECRETS START ###/,/### VAULT SECRETS END ###/d' .env; \
	cat -s .env > .env.tmp && mv .env.tmp .env; rm -f .env.bak

down: ## Stop running containers and delete volumes
	@rm -f dev-tools/compose/.env.findata
	@$(call docker_cmd,$(PROFILES),down --volumes)

get_env: clean ## Get latest secrets from vault, replacing old secrets
	@vault -v > /dev/null 2>&1 || { echo "⚠️  \033[31;1mVault CLI is not installed\033[0m ⚠️"; exit 1; }
	@printf "\n### VAULT SECRETS START ###\n" >> dev-tools/compose/.env
	@export VAULT_TOKEN=$$(vault login -method=oidc -token-only) && cd dev-tools/compose/; \
	$(call write_secret,KRAKEND_API_KEY,krakend/terminal-proxy,apikey) \
	$(call write_secret,C4GH_KEY,krakend/allas-encryption-key,key) \
	$(call write_secret,ARTIFACTORY_TOKEN,krakend/artifactory,token) \
	$(call write_secret,VAULT_ROLE,krakend/vault,role) \
	$(call write_secret,VAULT_SECRET,krakend/vault,secret) \
	$(call write_secret,FINDATA_ACCESS,krakend/findata,access) \
	$(call write_secret,FINDATA_SECRET,krakend/findata,secret) \
	$(call write_secret,FINDATA_S3_HOST,krakend/findata,host) \
	$(call write_secret,FINDATA_S3_REGION,krakend/findata,region) \
	$(call write_secret,FINDATA_BUCKET,krakend/findata,bucket) \
	$(call write_secret,ARTIFACTORY_SERVER,internal-urls,artifactory-docker) \
	$(call write_secret,ARTIFACTORY_URL,internal-urls,artifactory) \
	$(call write_secret,AAI_BASE_URL,internal-urls,test-aai) \
	$(call write_secret,S3_HOST,internal-urls,test-allas) \
	$(call write_secret,KRAKEND_ADDR,internal-urls,test-krakend-backend) \
	$(call write_secret,VALIDATOR_ADDR,internal-urls,test-krakend-backend) \
	$(call write_secret,KEYSTONE_BASE_URL,internal-urls,test-pouta)
	@set -a; source dev-tools/compose/.env; printf "BACKEND_HOST=$${KRAKEND_ADDR#*://}\n" >> dev-tools/compose/.env
	@printf "### VAULT SECRETS END ###\n" >> dev-tools/compose/.env
	@echo "Secrets written successfully"

run_profiles: down ## Run componets with possible profile arguments: fuse krakend keystone
	@if echo "$(RUN_ARGS)" | grep -q keystone; then \
		$(call docker_cmd,keystone-creds,up -d); \
	fi
	@$(MAKE) _wait_for_container CONTAINER_NAME=findata-creds
	$(call docker_cmd,$(RUN_ARGS),up)

build_profiles: down ## Build and run components with possible profile arguments: fuse krakend keystone
	$(call docker_cmd,$(RUN_ARGS),build)
	@$(MAKE) run_profiles RUN_ARGS="$(RUN_ARGS)"

exec: ## Access data-gateway container
	@trap 'exit 0' INT; docker exec -it data-gateway /bin/bash

envs:
	@echo "$$(grep -E 'PROXY_URL|SDS_ACCESS_TOKEN|CONFIG_ENDPOINT' dev-tools/compose/.env | xargs)"


### Following targets are for internal use, but you can still run them ###

_wait_for_container:
	@if [ -z `docker ps -a --format {{.Names}} --filter name=$(CONTAINER_NAME)` ]; then \
		sleep 2; \
	else \
		until [ "`docker inspect -f {{.State.Status}} $(CONTAINER_NAME)`" = "exited" ]; do \
			sleep 2; \
		done; \
	fi

UNAME := $(shell uname)
_follow_logs:
	@if [ "${UNAME}" = "Darwin" ]; then \
		osascript \
		-e 'tell application "Terminal"' \
			-e 'activate' \
			-e 'tell application "System Events" to keystroke "t" using command down' \
			-e 'do script "cd $(shell pwd); $(call docker_cmd,$(PROFILES),logs -f)" in front window' \
		-e 'end tell'; \
	elif command -v gnome-terminal >/dev/null; then \
		gnome-terminal --tab -- bash -c "$(call docker_cmd,$(PROFILES),logs -f); exec bash"; \
	elif command -v x-terminal-emulator >/dev/null; then \
		x-terminal-emulator -e bash -c "$(call docker_cmd,$(PROFILES),logs -f); exec bash"; \
	else \
		echo "⚠️  \033[31;1mFor logging, run '$(call docker_cmd,$(PROFILES),logs -f)' manually\033[0m ⚠️"; \
	fi
