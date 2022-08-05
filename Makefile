.PHONY: all
all: build-docker reconf

.PHONY: build-docker
build-docker:
	docker built -t takonobgp .

.PHONY: reconf
reconf:
	tinet reconf | sudo sh -ex
