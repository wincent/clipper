help:
	@echo 'make build   - build the clipper executable'
	@echo 'make tag     - tag the current HEAD with VERSION'
	@echo 'make archive - create an archive of the current HEAD for VERSION'
	@echo 'make upload  - upload the built archive of VERSION to Amazon S3'
	@echo 'make all     - build, tag, archive and upload VERSION'

version:
	@if [ "$$VERSION" = "" ]; then echo "VERSION not set"; exit 1; fi

clipper: clipper.go
	go build $^

build: clipper

tag: version
	git tag -s $$VERSION -m "$$VERSION release"

archive: clipper-$$VERSION.zip

clipper-$$VERSION.zip: version clipper
	git archive -o $@ HEAD
	zip $@ clipper

upload: version clipper-$$VERSION.zip
	aws put s3.wincent.com/clipper/releases/clipper-$$VERSION.zip clipper-$$VERSION.zip
	aws put "s3.wincent.com/clipper/releases/clipper-$$VERSION.zip?acl" --public

all: version build tag archive upload

.PHONY: clean
clean:
	rm -f clipper clipper-*.zip
