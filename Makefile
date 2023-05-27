VERSION := $(shell git describe --always --dirty)

ifdef STATIC
	CGO := CGO_ENABLED=0
endif

help:
	@echo 'make build   - build the clipper executable'
	@echo 'make tag     - tag the current HEAD with VERSION'
	@echo 'make archive - create an archive of the current HEAD for VERSION'
	@echo 'make upload  - upload the built archive of VERSION to Amazon S3'
	@echo 'make all     - build, tag, archive and upload VERSION'
	@echo ''
	@echo 'Append STATIC=1 to build a static binary.'

version:
	@if [ "$$VERSION" = "" ]; then echo "VERSION not set"; exit 1; fi

clipper_linux:
	$(CGO) GOOS=linux GOARCH=amd64 go build -ldflags="-X main.version=${VERSION}" -o clipper_linux clipper.go

clipper_darwin:
	$(CGO) GOOS=darwin GOARCH=amd64 go build -ldflags="-X main.version=${VERSION}" -o clipper_darwin clipper.go

clipper_all: clipper_linux clipper_darwin

clipper: clipper.go
	$(CGO) go build -ldflags="-X main.version=${VERSION}" $^

build: clipper

tag: version
	git tag -s ${VERSION} -m "${VERSION} release"

archive: clipper-${VERSION}.zip

clipper-${VERSION}.zip: clipper
	git archive -o $@ HEAD
	zip $@ clipper

upload: clipper-${VERSION}.zip
	aws --curl-options=--insecure put s3.wincent.com/clipper/releases/clipper-${VERSION}.zip clipper-${VERSION}.zip
	aws --curl-options=--insecure put "s3.wincent.com/clipper/releases/clipper-${VERSION}.zip?acl" --public

all: tag build archive upload

.PHONY: clean
clean:
	rm -f clipper clipper-*.zip
	rm -f clipper_*
