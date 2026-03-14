.PHONY: test test_js all

all: test test_js

test:
	go test ./...

testjs: 
	cd test_js && go run .. --dry-run --verbose
