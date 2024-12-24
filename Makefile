.PHONY: run test

all: test

test:
	CGO_ENABLED=0 go test -fullpath ./...

run: test
	CGO_ENABLED=0 go install ./cmd/bull
	sh -c ' \
	bull -bull_static=internal/html/ & \
	bull -bull_static=internal/html/ -content=$$HOME/hugo/content -listen=localhost:4444 & \
	bull -bull_static=internal/html/ -content=$$HOME/keep -listen=localhost:5555 -editor=textarea & \
	wait \
	'
