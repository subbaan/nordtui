.PHONY: build test

build:
	cd nordtui && go build -o nordtui .
	ln -sfn nordtui/nordtui nordtui-bin

test:
	cd nordtui && go test ./...
