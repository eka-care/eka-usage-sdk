.PHONY: all test-all lint-all build-all test-python test-ts test-go lint-python lint-ts lint-go build-python build-ts build-go clean

all: test-all

# ---------- aggregate ----------
test-all: test-python test-ts test-go
lint-all: lint-python lint-ts lint-go
build-all: build-python build-ts build-go

# ---------- Python ----------
test-python:
	cd sdks/python && python -m pytest -q

lint-python:
	cd sdks/python && ruff check eka_usage tests || true

build-python:
	cd sdks/python && python -m build

# ---------- TypeScript ----------
test-ts:
	cd sdks/typescript && npm test --silent

lint-ts:
	cd sdks/typescript && npm run lint --silent

build-ts:
	cd sdks/typescript && npm run build --silent

# ---------- Go ----------
test-go:
	cd sdks/go && go test ./...

lint-go:
	cd sdks/go && go vet ./...

build-go:
	cd sdks/go && go build ./...

clean:
	rm -rf sdks/python/dist sdks/python/build sdks/python/*.egg-info
	rm -rf sdks/typescript/dist sdks/typescript/node_modules
	cd sdks/go && go clean ./...
