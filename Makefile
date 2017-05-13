all: build

ENVVAR = GOOS=linux GOARCH=amd64 CGO_ENABLED=0
TAG = v0.1.0

build: clean
	$(ENVVAR) go build -o kubernetes-auto-ingress

container: build
	docker build -t kubernetes-auto-ingress:$(TAG) .

clean:
	rm -f kubernetes-auto-ingress

.PHONY: all build container clean
