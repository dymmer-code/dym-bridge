.PHONY: build test bundle bundle-arch bundle-assemble dmg dmg-package dmg-deps windows clean

APP_NAME = dym-bridge
BUNDLE_DIR = bin/$(APP_NAME).app
MIN_MACOS_VER = 11.0
MACOS_ENV = CGO_ENABLED=0 GOOS=darwin

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION_NUM = $(shell echo $(VERSION) | sed 's/^v//' | cut -d'-' -f1)

LDFLAGS = -s -w \
	-X github.com/dymmer-code/dym-bridge/pkg/version.Version=$(VERSION) \
	-X github.com/dymmer-code/dym-bridge/pkg/version.GitCommit=$(GIT_COMMIT) \
	-X github.com/dymmer-code/dym-bridge/pkg/version.BuildDate=$(BUILD_DATE)

build:
	@mkdir -p bin
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o bin/$(APP_NAME) ./cmd/dym-bridge

test:
	CGO_ENABLED=0 go test ./...
	CGO_ENABLED=0 go vet ./...

bundle:
	@echo "Building Universal binary for macOS $(MIN_MACOS_VER)+ (arm64 + amd64) [Version: $(VERSION)]..."
	@mkdir -p bin
	@$(MACOS_ENV) GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o bin/$(APP_NAME)_arm64 ./cmd/dym-bridge
	@$(MACOS_ENV) GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o bin/$(APP_NAME)_amd64 ./cmd/dym-bridge
	@lipo -create -output bin/$(APP_NAME) bin/$(APP_NAME)_amd64 bin/$(APP_NAME)_arm64
	@rm -f bin/$(APP_NAME)_arm64 bin/$(APP_NAME)_amd64
	@$(MAKE) --no-print-directory bundle-assemble

# Puts bin/dym-bridge (already built) in the .app with its Info.plist.
bundle-assemble:
	@echo "Bundling $(BUNDLE_DIR)..."
	@rm -rf $(BUNDLE_DIR)
	@mkdir -p $(BUNDLE_DIR)/Contents/MacOS
	@mkdir -p $(BUNDLE_DIR)/Contents/Resources
	@cp bin/$(APP_NAME) $(BUNDLE_DIR)/Contents/MacOS/$(APP_NAME)
	@if [ -f dym-bridge.icns ]; then cp dym-bridge.icns $(BUNDLE_DIR)/Contents/Resources/dym-bridge.icns; fi
	@printf '%s\n' \
		'<?xml version="1.0" encoding="UTF-8"?>' \
		'<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">' \
		'<plist version="1.0">' \
		'<dict>' \
		'    <key>CFBundleExecutable</key>' \
		'    <string>$(APP_NAME)</string>' \
		'    <key>CFBundleIconFile</key>' \
		'    <string>dym-bridge.icns</string>' \
		'    <key>CFBundleIdentifier</key>' \
		'    <string>com.dymmer.bridge</string>' \
		'    <key>CFBundleName</key>' \
		'    <string>Dym Bridge</string>' \
		'    <key>CFBundleDisplayName</key>' \
		'    <string>Dym Bridge</string>' \
		'    <key>CFBundlePackageType</key>' \
		'    <string>APPL</string>' \
		'    <key>CFBundleShortVersionString</key>' \
		'    <string>$(VERSION_NUM)</string>' \
		'    <key>CFBundleVersion</key>' \
		'    <string>$(VERSION_NUM)</string>' \
		'    <key>LSMinimumSystemVersion</key>' \
		'    <string>$(MIN_MACOS_VER)</string>' \
		'    <key>NSHighResolutionCapable</key>' \
		'    <true/>' \
		'    <key>CFBundleURLTypes</key>' \
		'    <array>' \
		'        <dict>' \
		'            <key>CFBundleURLName</key>' \
		'            <string>Dym Bridge sign-in</string>' \
		'            <key>CFBundleURLSchemes</key>' \
		'            <array>' \
		'                <string>dym-bridge</string>' \
		'            </array>' \
		'        </dict>' \
		'    </array>' \
		'</dict>' \
		'</plist>' > $(BUNDLE_DIR)/Contents/Info.plist
	@codesign --force --sign - "$(BUNDLE_DIR)"
	@echo "✔ Universal $(BUNDLE_DIR) created for macOS $(MIN_MACOS_VER)+!"

# One architecture only (CI builds each on its own runner): the same bundle as `bundle`, without the universal merge.
bundle-arch:
	@test -n "$(ARCH)" || (echo "ARCH=arm64|amd64 is required" && exit 1)
	@mkdir -p bin
	@$(MACOS_ENV) GOARCH=$(ARCH) go build -ldflags="$(LDFLAGS)" -o bin/$(APP_NAME) ./cmd/dym-bridge
	@$(MAKE) --no-print-directory bundle-assemble

DMG_PYTHON ?= $(CURDIR)/bin/dmg-venv/bin/python

dmg-deps:
	python3 -m venv bin/dmg-venv
	bin/dmg-venv/bin/python -m pip install -r packaging/macos/dmg/requirements.txt

dmg: bundle
	$(MAKE) --no-print-directory dmg-package

dmg-package:
	DMG_PYTHON="$(DMG_PYTHON)" bash packaging/macos/create-dmg.sh "$(BUNDLE_DIR)" "bin/$(APP_NAME).dmg"

windows:
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS) -H=windowsgui" -o bin/$(APP_NAME).exe ./cmd/dym-bridge

clean:
	rm -rf bin/ cmd/generate-icons

# Isolated GoGPU experiment: no Fyne imports and no CGo.
.PHONY: preview-build preview preview-test preview-bundle
preview-build:
	CGO_ENABLED=0 go build -o bin/dym-bridge-gpu ./cmd/dym-bridge-gpu

preview: preview-build
	./bin/dym-bridge-gpu

preview-test:
	CGO_ENABLED=0 go test ./pkg/gpuui ./pkg/sites ./pkg/config ./pkg/provision
	CGO_ENABLED=0 go vet ./pkg/gpuui ./pkg/sites ./pkg/config ./pkg/provision ./cmd/dym-bridge-gpu

preview-bundle: preview-build
	sh packaging/macos/preview.sh
