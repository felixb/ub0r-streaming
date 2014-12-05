.PHONY: all build-all clean dist

all:
	make -C go

build-all:
	make -C go build-all

dist: build-all
	-rm -r dist
	mkdir -p dist/usr/local/ub0r-streaming/bin dist/usr/local/bin
	cp -r html dist/usr/local/ub0r-streaming/static
	cp go/rtp-config dist/usr/local/ub0r-streaming/bin/
	cp go/rtp-receiver dist/usr/local/ub0r-streaming/bin/
	cp go/rtp-sender dist/usr/local/ub0r-streaming/bin/
	ln -s ../ub0r-streaming/rtp-config dist/usr/local/bin/rtp-config
	ln -s ../ub0r-streaming/rtp-receiver dist/usr/local/bin/rtp-receiver
	ln -s ../ub0r-streaming/rtp-sender dist/usr/local/bin/rtp-sender
	tar -czPp --xform 's:^dist/:/:' -f ub0r-streaming.tar.gz dist/*

clean:
	make -C go clean
	-rm -r *tar.gz dist