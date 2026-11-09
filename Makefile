.PHONY: test test-ui test-native race vet build package check licenses
export NUDILO_TEST_ARTIFACTS ?= $(CURDIR)/build/test-artifacts

test:
	CGO_ENABLED=0 go test ./...
	python3 -m unittest discover -s scripts -p 'test_*.py'

test-ui:
	CGO_ENABLED=0 go test -count=1 ./internal/desktop ./internal/locale

test-native:
	go run ./cmd/callbell-assets
	./scripts/prepare-native-test.sh

race:
	go test -race ./internal/...

vet:
	go vet ./...
	go tool mygo vet .

build:
	CGO_ENABLED=0 go build ./...

licenses:
	go run ./cmd/callbell-assets -check-licenses

check: test vet licenses
	python3 scripts/release.py check

package:
	./scripts/package.sh $(ARGS)
