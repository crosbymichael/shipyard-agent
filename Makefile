all:
	@go get -d -v ./...
	@cd ./agent && go build -o ../shipyard-agent

fmt:
	@cd ./agent && go fmt
clean:
	@rm -rf shipyard-agent
