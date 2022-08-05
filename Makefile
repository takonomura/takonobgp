.PHONY: all
all: build-docker reconf

.PHONY: build-docker
build-docker:
	docker build -t takonobgp .

.PHONY: reconf
reconf:
	tinet reconf | sed -E 's/--rm //' | sudo sh -ex
