.PHONY: build test bundle bundle-arch dmg windows clean

APP_NAME = dym-bridge
BUNDLE_DIR = bin/$(APP_NAME).app
MIN_MACOS_VER = 11.0
MACOS_ENV = CGO_ENABLED=1 GOOS=darwin MACOSX_DEPLOYMENT_TARGET=$(MIN_MACOS_VER) CGO_CFLAGS="-mmacosx-version-min=$(MIN_MACOS_VER)" CGO_LDFLAGS="-mmacosx-version-min=$(MIN_MACOS_VER)"

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
	$(MACOS_ENV) go build -ldflags="$(LDFLAGS)" -o bin/$(APP_NAME) ./cmd/dym-bridge

test:
	go test -v ./...

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
		'    <string>$(VERSION)</string>' \
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
	@echo "✔ Universal $(BUNDLE_DIR) created for macOS $(MIN_MACOS_VER)+!"

# One architecture only (CI builds each on its own runner): the same bundle as `bundle`, without the universal merge.
bundle-arch:
	@test -n "$(ARCH)" || (echo "ARCH=arm64|amd64 is required" && exit 1)
	@mkdir -p bin
	@$(MACOS_ENV) GOARCH=$(ARCH) go build -ldflags="$(LDFLAGS)" -o bin/$(APP_NAME) ./cmd/dym-bridge
	@$(MAKE) --no-print-directory bundle-assemble

dmg: bundle
	@echo "Creating DMG..."
	@rm -rf bin/dmg_staging bin/$(APP_NAME).dmg bin/$(APP_NAME)-temp.dmg
	@mkdir -p bin/dmg_staging
	@cp -R $(BUNDLE_DIR) bin/dmg_staging/
	@if [ -f dym-bridge.ini.example ]; then cp dym-bridge.ini.example bin/dmg_staging/; fi
	@hdiutil create -volname "$(APP_NAME)" -srcfolder bin/dmg_staging -ov -format UDRW bin/$(APP_NAME)-temp.dmg
	@rm -rf bin/dmg_staging
	@DEVICE=$$(hdiutil attach -readwrite -noverify -noautoopen bin/$(APP_NAME)-temp.dmg | egrep '^/dev/' | sed 1q | awk '{print $$1}'); \
	cd /Volumes/$(APP_NAME) && ln -s /Applications && cd -; \
	hdiutil detach "$$DEVICE"
	@hdiutil convert bin/$(APP_NAME)-temp.dmg -format UDZO -o bin/$(APP_NAME).dmg
	@rm -f bin/$(APP_NAME)-temp.dmg
	@echo "✔ bin/$(APP_NAME).dmg created!"

windows:
	@echo "Building Windows executable bin/$(APP_NAME).exe..."
	@mkdir -p bin
	@if command -v zig >/dev/null 2>&1; then \
		CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC="zig cc -target x86_64-windows-gnu" CXX="zig c++ -target x86_64-windows-gnu" go build -ldflags="$(LDFLAGS) -H=windowsgui" -o bin/$(APP_NAME).exe ./cmd/dym-bridge; \
	elif command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then \
		CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc go build -ldflags="$(LDFLAGS) -H=windowsgui" -o bin/$(APP_NAME).exe ./cmd/dym-bridge; \
	else \
		GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS) -H=windowsgui" -o bin/$(APP_NAME).exe ./cmd/dym-bridge; \
	fi
	@echo "✔ bin/$(APP_NAME).exe built!"

clean:
	rm -rf bin/ cmd/generate-icons

# Isolated GoGPU experiment: no Fyne imports and no CGo.
.PHONY: preview-build preview preview-test preview-bundle
preview-build:
	CGO_ENABLED=0 go build -o bin/dym-bridge-gpu ./cmd/dym-bridge-gpu

preview: preview-build
	./bin/dym-bridge-gpu

preview-test:
	CGO_ENABLED=0 go test ./pkg/gpuui ./pkg/sites
	CGO_ENABLED=0 go vet ./pkg/gpuui ./pkg/sites ./cmd/dym-bridge-gpu

preview-bundle: preview-build
	sh packaging/macos/preview.sh
