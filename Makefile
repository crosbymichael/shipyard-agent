all:
	@go get github.com/fsouza/go-dockerclient
	@cd ./agent && go build -o ../shipyard-agent

fmt:
	@cd ./agent && go fmt
clean:
	@rm -rf shipyard-agent
