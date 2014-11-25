.PHONY: all rhel7

all: gbalancer

GOPATH = $(PWD)/build
GOBIN = 
export GOPATH

URL = github.com/zhgwenming
REPO = gbalancer

URLPATH = $(GOPATH)/src/$(URL)

gbalancer: engine/native/*.go
	@[ -d $(URLPATH) ] || mkdir -p $(URLPATH)
	@ln -nsf $(PWD) $(URLPATH)/$(REPO)
	go install $(URL)/$(REPO)/cmd/gbalancer $(URL)/$(REPO)/cmd/streamd

rhel7: galerabalancer

galerabalancer:
	go install -compiler gccgo $(URL)/$(REPO)/cmd/gbalancer $(URL)/$(REPO)/cmd/streamd

clean:
	rm -fv build/bin/*
	rm -fv lb cmd/gbalancer/gbalancer galerabalancer gbalancer
