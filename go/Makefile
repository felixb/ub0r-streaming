.PHONY: all clean get
SOURCES=rtp-config.go rtp-receiver.go rtp-sender.go common.go common-client.go common-sender.go
EXECUTABLES=rtp-config rtp-receiver rtp-sender

all: get build-all

build-all: $(EXECUTABLES)

get:
	go get -d -a .

rtp-config: rtp-config.go common.go common-client.go common-sender.go
	go build -o $@ $^

rtp-receiver: rtp-receiver.go common.go common-client.go
	go build -o $@ $^

rtp-sender: rtp-sender.go common.go common-client.go common-sender.go
	go build -o $@ $^

clean:
	-rm -rf dist $(EXECUTABLES)
