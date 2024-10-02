# eBPF Makefile for handling eBPF-related tasks

# Variables
CLANG := clang
BPFTOOL := bpftool
SRC_DIR := c/src
OBJ_DIR := c/obj
INTERFACE := eth0 # Replace with the network interface you want to attach to

# Source and object files
SOURCES := $(wildcard $(SRC_DIR)/*.c)
OBJECTS := $(patsubst $(SRC_DIR)/%.c, $(OBJ_DIR)/%.o, $(SOURCES))

# eBPF targets
.PHONY: ebpf-all ebpf-build ebpf-build-test ebpf-load ebpf-load-test ebpf-unload ebpf-status ebpf-clean

# Default eBPF target
ebpf-all: ebpf-build

# Create object directory if it doesn't exist
$(OBJ_DIR):
	mkdir -p $(OBJ_DIR)

# Compile all eBPF programs
$(OBJ_DIR)/%.o: $(SRC_DIR)/%.c | $(OBJ_DIR)
	$(CLANG) -O2 -g -target bpf -c $< -o $@

# Build the production eBPF program
ebpf-build: $(OBJ_DIR)/ebpf_program.o
	@echo "Production eBPF program compiled successfully."

# Build the test eBPF program
ebpf-build-test: $(OBJ_DIR)/ebpf_test_program.o
	@echo "Test eBPF program compiled successfully."

# Load the production eBPF program onto the interface
ebpf-load: $(OBJ_DIR)/ebpf_program.o
	sudo ip link set dev $(INTERFACE) xdp obj $(OBJ_DIR)/ebpf_program.o sec xdp_router_func
	@echo "Production eBPF program loaded onto interface $(INTERFACE)."

# Load the test eBPF program onto the interface
ebpf-load-test: $(OBJ_DIR)/ebpf_test_program.o
	sudo ip link set dev $(INTERFACE) xdp obj $(OBJ_DIR)/ebpf_test_program.o sec xdp_pass_func
	@echo "Test eBPF program loaded onto interface $(INTERFACE)."

# Unload the eBPF program from the interface
ebpf-unload:
	sudo ip link set dev $(INTERFACE) xdp off
	@echo "eBPF program unloaded from interface $(INTERFACE)."

# Check if the eBPF program is loaded
ebpf-status:
	sudo $(BPFTOOL) prog list

# Clean up the compiled objects
ebpf-clean:
	rm -rf $(OBJ_DIR)
	@echo "Cleaned up eBPF compiled files."
