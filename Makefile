#!make

# compute dynamic values
COMMIT := $(shell git rev-parse --short HEAD)
DATE   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION ?= dev

# production build for Windows 64-bit amd64  -H=windowsgui
prod:
	@echo "Building Windows amd64 executable..."
	@GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -H=windowsgui \
		-X 'main.Version=$(VERSION)' \
		-X 'main.Commit=$(COMMIT)' \
		-X 'main.BuildDate=$(DATE)'" \
	-o elkonexport.exe .

# production build for Windows 32-bit amd  -H=windowsgui
prod386:
	@echo "Building Windows x386 executable..."
	@GOOS=windows GOARCH=386 go build -ldflags "-s -w -H=windowsgui \
		-X 'main.Version=$(VERSION)' \
		-X 'main.Commit=$(COMMIT)' \
		-X 'main.BuildDate=$(DATE)'" \
	-o elkonexport.exe .

debug:
	@echo "Building Windows debug amd64 executable..."
	@GOOS=windows GOARCH=amd64 go build -ldflags "-s -w \
		-X 'main.Version=$(VERSION)' \
		-X 'main.Commit=$(COMMIT)' \
		-X 'main.BuildDate=$(DATE)'" \
	-o elkonexport.exe .

debug386:
	@echo "Building Windows debug x386 executable..."
	@GOOS=windows GOARCH=386 go build -ldflags "-s -w \
		-X 'main.Version=$(VERSION)' \
		-X 'main.Commit=$(COMMIT)' \
		-X 'main.BuildDate=$(DATE)'" \
	-o elkonexport.exe .
%:
	@:


