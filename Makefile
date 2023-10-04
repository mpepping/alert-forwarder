PROJECT := alert-forwarder
REGISTRY ?= docker-registry.example.com
IMAGE := $(REGISTRY)/$(PROJECT)
SRCDIRS := ./cmd/alert-forwarder
PKGS := $(shell go list ./cmd/...)

VERSION ?= 1.0.4

all: build install

build:
	go build ./...

install:
	go install -v ./cmd/alert-forwarder/...

gofmt:
	@echo Checking code is gofmted
	@test -z "$(shell gofmt -s -l -d -e $(SRCDIRS) | tee /dev/stderr)"

image:
	docker build . -t $(IMAGE):$(VERSION)

push:
	docker push $(IMAGE):$(VERSION)

.PHONY: all
