.DEFAULT_GOAL := run
SWAG_VERSION := v1.16.4
.PHONY: run test swagger

run:
	go run ./cmd/api
test:
	go test ./...

swagger:
	go run github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION) init \
		-g cmd/api/main.go \
		-o docs \
		--parseInternal
