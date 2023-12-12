build-plugins:
	CGO_ENABLED=$(CGO_ENABLED) CC=$(CC) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -buildmode=plugin -o $(DEST)/command_pubsub.so ./src/plugins/pubsub/*.go

build-server:
	CGO_ENABLED=$(CGO_ENABLED) CC=$(CC) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -o $(DEST)/server ./src/*.go

build:
	env CGO_ENABLED=1 CC=x86_64-linux-musl-gcc GOOS=linux GOARCH=amd64 DEST=bin/linux/x86_64/plugins make build-plugins
	env CGO_ENABLED=1 CC=x86_64-linux-musl-gcc GOOS=linux GOARCH=amd64 DEST=bin/linux/x86_64 make build-server
