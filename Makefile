build: build-windows-amd64 build-linux-amd64
build-%: install-deps
	$(eval OSARCH = $(subst -, ,$*))
	$(eval export GOOS = $(word 1,$(OSARCH)))
	$(eval export GOARCH = $(word 2,$(OSARCH)))
	@echo Building $*
	go build -ldflags "-s -w$(if $(filter windows,$(GOOS)), -H windowsgui,)" -trimpath -o out/$(GOOS)-$(GOARCH)/ExcelSplitter$(if $(filter windows,$(GOOS)),.exe,)
	cp LICENSE COPYRIGHT NOTICE README.md out/$(GOOS)-$(GOARCH)/

install-deps:
ifeq ($(shell uname),Linux)
	@echo Installing Linux build dependencies
	sudo apt install gcc pkg-config libwayland-dev libx11-dev libx11-xcb-dev libxkbcommon-x11-dev libgles2-mesa-dev libegl1-mesa-dev libffi-dev libxcursor-dev libvulkan-dev
endif

dev:
	nodemon --signal SIGKILL --ext go --exec "(go build -o out\dev.exe && out\dev.exe) || exit 1"
