.PHONY: run test

all: install

test:
	CGO_ENABLED=0 go test -fullpath ./...

install: test
	CGO_ENABLED=0 go install ./cmd/bull

run: install
	sh -c ' \
	bull serve -bull_static=internal/assets/ & \
	bull -content=$$HOME/hugo/content serve -bull_static=internal/assets/ -listen=localhost:4444 & \
	bull -content ~/keep serve -bull_static=internal/assets/ -listen=localhost:5555 -editor=textarea & \
	wait \
	'
