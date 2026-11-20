.PHONY: run build icon clean

BIN := bin/v2ex_mygo.exe
ICON := assets/img/app.ico
SYSO := rsrc_windows_amd64.syso

run:
	@go run main.go

# icon writes the Windows resource go build embeds, so the exe carries the
# app logo in Explorer and on the taskbar.
icon:
	@go run github.com/akavel/rsrc@v0.10.2 -ico $(ICON) -o $(SYSO)

build: icon
	@go build -o $(BIN) .

clean:
	@rm -rf bin
