.PHONY: all rhel7 builddir

all: gbalancer

GOPATH = $(PWD)/_build
GOBIN = 
export GOPATH

URL = github.com/zhgwenming
REPO = gbalancer

URLPATH = $(GOPATH)/src/$(URL)

builddir:
	@[ -d $(URLPATH) ] || mkdir -p $(URLPATH)
	@ln -nsf $(PWD) $(URLPATH)/$(REPO)

gbalancer: engine/native/*.go builddir
	go install $(URL)/$(REPO) $(URL)/$(REPO)/cmd/streamd

rhel7: $(GOPATH)/bin/galerabalancer

$(GOPATH)/bin/galerabalancer: *.go builddir
	go build -compiler gccgo -o $@

clean:
	rm -rf _build
	rm -fv lb cmd/gbalancer/gbalancer galerabalancer gbalancer
