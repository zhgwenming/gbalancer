.PHONY: all rhel7 builddir

all: gbalancer

GOPATH = $(PWD)/_build
GOBIN = 
export GOPATH
VERSION = $(shell git describe)

URL = github.com/zhgwenming
REPO = gbalancer

URLPATH = $(GOPATH)/src/$(URL)

builddir:
	@[ -d $(URLPATH) ] || mkdir -p $(URLPATH)
	@ln -nsf $(PWD) $(URLPATH)/$(REPO)

gbalancer: engine/native/*.go builddir
	go install -ldflags "-X main.VERSION $(VERSION)" $(URL)/$(REPO) $(URL)/$(REPO)/cmd/streamd

rhel7: $(GOPATH)/bin/galerabalancer

$(GOPATH)/bin/galerabalancer: *.go builddir
	go install -compiler gccgo $(URL)/$(REPO)/cmd/gbalancer $(URL)/$(REPO)/cmd/streamd

clean:
	rm -rf _build
	rm -fv lb cmd/gbalancer/gbalancer galerabalancer gbalancer
