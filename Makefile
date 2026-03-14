.PHONY: test dry-run

all: test test_js

test:
	go test ./...

testjs: 
	cd test_js && go run .. --dry-run --verbose
