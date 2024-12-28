.PHONY: run test

all: install

test:
	CGO_ENABLED=0 go test -fullpath ./...

install: test
	CGO_ENABLED=0 go install ./cmd/bull

run: install
	sh -c ' \
	bull -bull_static=internal/html/ & \
	bull -bull_static=internal/html/ -content=$$HOME/hugo/content -listen=localhost:4444 & \
	bull -bull_static=internal/html/ -content=$$HOME/keep -listen=localhost:5555 -editor=textarea & \
	wait \
	'
