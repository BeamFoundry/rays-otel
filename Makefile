.PHONY: all
all: test

.PHONY: fmt
fmt:
	go vet ./...
	go fmt ./...

.PHONY: fix
fix: fmt
	go fix ./...

.PHONY: clean
clean:
	rm -f coverprofile.out

.PHONY: test
test: fmt
	# Requires golangci-lint v2.15.0+ (tested minimum supporting Go 1.27)
	golangci-lint run ./...
	go run github.com/onsi/ginkgo/v2/ginkgo run -cover ./...

.PHONY: validate
validate:
	$(MAKE) fix
	$(MAKE) test
