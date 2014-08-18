.PHONY: all rhel7

all: gbalancer

PKGPREFIX = github.com/zhgwenming
PKGNAME = $(PKGPREFIX)/gbalancer

SRCDIR = $(PWD)/build/src/$(PKGPREFIX)

gbalancer: engine/native/*.go
	#cd cmd/gbalancer && go build -o $@
	@[ -d $(SRCDIR) ] || mkdir -p $(SRCDIR)
	@ln -nsf $(PWD) $(SRCDIR)/
	GOPATH=$(PWD)/build:$(PWD)/Godeps/_workspace 	\
		go build  -o $@ $(PKGNAME)/cmd/gbalancer

rhel7: galerabalancer

galerabalancer: *.go
	go build -compiler gccgo -o $@

clean:
	rm -fv lb cmd/gbalancer/gbalancer galerabalancer gbalancer
