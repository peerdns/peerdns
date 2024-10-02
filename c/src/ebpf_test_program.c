#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/udp.h>
#include <linux/tcp.h>

struct route_key {
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
    __u32 pad; // Padding to align to 16 bytes
};

struct route_value {
    __u8 next_hop_mac[6];
    __u16 pad; // Padding to align to 8 bytes
};

// Define the routes_map as in the production eBPF program
struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, struct route_key);
    __type(value, struct route_value);
} routes_map SEC(".maps");

// The test XDP program
SEC("xdp_test_pass_func")
int xdp_program(struct xdp_md *ctx) {
    // For testing purposes, simply pass the packet without any processing
    return XDP_PASS;
}

char _license[] SEC("license") = "GPL";