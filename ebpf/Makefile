CLANG   ?= clang
LLC     ?= llc
BPFTOOL ?= bpftool

ARCH    := $(shell uname -m | sed 's/x86_64/x86/' | sed 's/aarch64/arm64/')
CFLAGS   = -O2 -g -target bpf -D__TARGET_ARCH_$(ARCH) \
           -I/usr/include/$(shell uname -m)-linux-gnu \
           -Wall -Werror

PROGS = tcp_trace packet_filter latency_probe syscall_monitor dns_tracker \
        nccl_trace gpu_mem_trace rdma_trace training_pattern datapipe_bottleneck gradient_compress \
        container_escape crypto_detect exfil_detect privesc_monitor driver_fim \
        tcp_tuning connpool_analyze numa_path sockops_optimize trace_correlator \
        latency_breakdown cost_tracker fingerprint

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

# Security programs depend on security_common.h as well.
container_escape.o crypto_detect.o exfil_detect.o privesc_monitor.o driver_fim.o: headers/security_common.h

# GPU/AI programs depend on gpu_common.h.
nccl_trace.o gpu_mem_trace.o rdma_trace.o gradient_compress.o: headers/gpu_common.h
training_pattern.o datapipe_bottleneck.o: headers/gpu_common.h

clean:
	rm -f *.o headers/vmlinux.h
