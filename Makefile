BUILD_VERSION 	:= $(shell git describe --tags 2>/dev/null)
ifeq ($(BUILD_VERSION),)
	BUILD_VERSION = git-$(shell git rev-parse --short HEAD)
endif

LDFLAGS := -ldflags '-X main.BuildVersion=$(BUILD_VERSION)'

all: vc

clean:
	$(RM) vc

vc:
	go build $(LDFLAGS) ./cmd/vc
