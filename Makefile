.PHONY: test testjs testjspatch all

all: test testjs testjspatch

test:
	go test ./...

testjs: 
	cd test_js && go run .. --dry-run --verbose

testjspatch:
	cd test_js && go run .. --dry-run --verbose --patch-only