CLANG   ?= clang
LLC     ?= llc
BPFTOOL ?= bpftool

ARCH    := $(shell uname -m | sed 's/x86_64/x86/' | sed 's/aarch64/arm64/')
CFLAGS   = -O2 -g -target bpf -D__TARGET_ARCH_$(ARCH) \
           -Wall -Werror

PROGS = tcp_trace packet_filter latency_probe syscall_monitor dns_tracker

.PHONY: all clean vmlinux

all: $(PROGS:%=%.o)

# Generate vmlinux.h from the running kernel if bpftool is available.
headers/vmlinux.h:
	@if command -v $(BPFTOOL) >/dev/null 2>&1; then \
		$(BPFTOOL) btf dump file /sys/kernel/btf/vmlinux format c > $@; \
	else \
		echo "WARNING: bpftool not found, skipping vmlinux.h generation"; \
		touch $@; \
	fi

%.o: %.c headers/common.h
	$(CLANG) $(CFLAGS) -c $< -o $@

clean:
	rm -f *.o headers/vmlinux.h
