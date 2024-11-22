
#  r release download 12 -d ./release.tgz
#  make test-embed ./release.tgz
.PHONY: test-embed
test-embed: bin/installer
	go run ./hack/dev-embed.go ./bin/installer $(release) ./bin/installer
