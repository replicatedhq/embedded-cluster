SHELL := /bin/bash

.PHONY: swagger
swagger:
	swag fmt -g api.go
	swag init -g api.go

.PHONY: swag
swag:
	which swag || (go install github.com/swaggo/swag/cmd/swag@latest)
