.PHONY: run build clean

BIN := bin/v2ex_mygo.exe

run:
	@go run main.go

build:
	@go build -o $(BIN) .

clean:
	@rm -rf bin
