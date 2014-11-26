.PHONY: all clean get
SOURCES=rtp-config.go rtp-receiver.go rtp-sender.go common.go common-client.go
EXECUTABLES=rtp-config rtp-receiver rtp-sender

all: get build-all

build-all: $(EXECUTABLES)

get:
	go get -d -a .

rtp-config: rtp-config.go common.go
	go build -o $@ $^

rtp-receiver: rtp-receiver.go common.go common-client.go
	go build -o $@ $^

rtp-sender: rtp-sender.go common.go common-client.go
	go build -o $@ $^

dist: build-all
	-mkdir -p dist/usr/local/ub0r-streaming/bin dist/usr/local/bin
	cp -r static dist/usr/local/ub0r-streaming/
	cp $(EXECUTABLES) dist/usr/local/ub0r-streaming/bin/
	ln -s ../ub0r-streaming/rtp-config dist/usr/local/bin/rtp-config
	ln -s ../ub0r-streaming/rtp-receiver dist/usr/local/bin/rtp-receiver
	ln -s ../ub0r-streaming/rtp-sender dist/usr/local/bin/rtp-sender
	tar -czPp --xform 's:^dist/:/:' -f ub0r-streaming.tar.gz dist/*

clean:
	-rm -rf dist ub0r-streaming.tar.gz $(EXECUTABLES)
