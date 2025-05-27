SHELL := /bin/bash

.PHONY: swagger
swagger:
	swag fmt -g api.go
	swag init --v3.1 -g api.go

.PHONY: swag
swag:
	which swag || (go install github.com/swaggo/swag/v2/cmd/swag@latest)
