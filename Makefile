.PHONY: test dry-run

test:
	go test ./...

testjs: 
	cd test_js && go run .. --dry-run && cd ..
