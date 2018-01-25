VERSION = $(shell grep 'version =' version.go | sed -E 's/.*"(.+)"$$/\1/')

all: build

deps:
	go get -u github.com/quickfixgo/quickfix
	go get -u github.com/shopspring/decimal

build: deps
	go install ./cmd/...

version:
	@echo $(VERSION)

.PTHONY: all deps build version