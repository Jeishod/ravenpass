# Ravenpass. Run `make help` for the targets; each app's Makefile has its own.

.DEFAULT_GOAL := help
.PHONY: help deps generate messages check check-core check-ui fmt-check licenses-check \
	desktop desktop-dev desktop-check \
	android android-dev android-run android-check \
	extension extension-dev extension-check clean

ROOT := $(CURDIR)
CORE_MODULES := packages/vault packages/authenticator packages/importers
GO_MODULES := $(CORE_MODULES) packages/app apps/desktop apps/mobile

include notices/notices.mk

help:
	@echo "Ravenpass $$(cat version.txt)"
	@echo
	@echo "  make deps             install npm dependencies"
	@echo "  make check            every check: core, interface, extension, macOS, Android"
	@echo "  make check-core       tidy-check every Go module; vet and race-test the platform-independent ones"
	@echo "  make check-ui         lint, type-check and test the shared interface"
	@echo "  make fmt-check        fail on Go files that gofmt would change"
	@echo "  make licenses-check   fail on a Go, npm or Android dependency outside the license allowlist"
	@echo "  make generate         regenerate the committed bindings and native string catalogs"
	@echo "  make messages         regenerate only the Go services' string catalog"
	@echo "  make clean            remove every app's build output"
	@echo
	@echo "  make desktop          ad-hoc signed macOS app in apps/desktop/bin"
	@echo "  make desktop-dev      run the macOS app with live reload"
	@echo "  make desktop-check    build, vet and test the macOS app"
	@echo "  make android          debug APK in apps/mobile/build/android/app/build/outputs/apk/debug"
	@echo "  make android-dev      run on the connected phone with live reload"
	@echo "  make android-run      install and start the debug APK on the connected phone"
	@echo "  make android-check    vet and test, then build and lint the debug APK"
	@echo "  make extension        Chrome extension zips in apps/extension/dist"
	@echo "  make extension-dev    run Chrome with the extension and live reload"
	@echo "  make extension-check  lint, type-check, test and build the Chrome extension"
	@echo
	@echo "Release and signing targets live in apps/desktop, apps/mobile and apps/extension."

deps:
	npm ci

generate: messages
	$(MAKE) -C apps/desktop bindings
	$(MAKE) -C apps/mobile strings

# The Go services' wording: the interface catalogs' messages that names.json lists.
messages: node_modules
	node packages/ui/scripts/export-messages.ts --values packages/app/messages/names.json \
		> packages/app/messages/catalog.generated.json

check: check-core check-ui licenses-check extension-check desktop-check android-check

fmt-check:
	@test -z "$$(gofmt -l packages apps)" || { gofmt -l packages apps; exit 1; }

check-core: fmt-check
	@for module in $(GO_MODULES); do \
		(cd $$module && go mod tidy -diff) || { echo "run go mod tidy in $$module"; exit 1; }; \
	done
	@for module in $(CORE_MODULES); do \
		(cd $$module && go vet ./... && go test -race ./...) || exit 1; \
	done

node_modules: package-lock.json
	npm ci
	@touch node_modules

check-ui: node_modules
	npm --prefix packages/ui run typecheck
	npm --prefix packages/ui run check
	npm --prefix packages/ui test

licenses-check: node_modules
	$(npm-licenses-check)
	$(MAKE) -C apps/desktop licenses-check
	$(MAKE) -C apps/mobile licenses-check

desktop:
	$(MAKE) -C apps/desktop package

desktop-dev:
	$(MAKE) -C apps/desktop dev

desktop-check:
	$(MAKE) -C apps/desktop check

android:
	$(MAKE) -C apps/mobile apk

android-dev:
	$(MAKE) -C apps/mobile dev

android-run:
	$(MAKE) -C apps/mobile run

android-check:
	$(MAKE) -C apps/mobile check

extension:
	$(MAKE) -C apps/extension package

extension-dev:
	$(MAKE) -C apps/extension dev

extension-check:
	$(MAKE) -C apps/extension check

clean:
	$(MAKE) -C apps/desktop clean
	$(MAKE) -C apps/mobile clean
	$(MAKE) -C apps/extension clean
