NPM ?= npm

.PHONY: deps build test package clean validate

deps:
	$(NPM) install

validate:
	$(NPM) run validate:manifest

build:
	$(NPM) run build

test:
	$(NPM) run test

package:
	$(NPM) run package

clean:
	$(NPM) run clean
