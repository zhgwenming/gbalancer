.PHONY: all rhel7

all: gbalancer

gbalancer: engine/native/*.go
	cd cmd/gbalancer && go build -o $@

rhel7: galerabalancer

galerabalancer: *.go
	go build -compiler gccgo -o $@

clean:
	rm -fv lb gbalancer galerabalancer
