.PHONY: test testjs all

all: test testjs

test:
	go test ./...

testjs: 
	cd test_js && go run .. --dry-run --verbose
