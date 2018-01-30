TAG=$(shell git rev-parse --short HEAD)

image:
	mkdir -p bin
	CGO_ENABLED=0 go build -o bin/k8s2slack *.go
	cp /etc/ssl/certs/ca-certificates.crt bin/ #Because otherwise x509 wont work in scratch image
	docker build -t $(PREFIX)k8s2slack .
ifneq ("$(PREFIX)","")
	docker push $(PREFIX)k8s2slack:latest
	docker tag $(PREFIX)k8s2slack $(PREFIX)k8s2slack:$(TAG)
	docker push $(PREFIX)k8s2slack:$(TAG)
endif
