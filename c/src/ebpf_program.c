#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/udp.h>
#include <linux/tcp.h>
#include <linux/in.h>

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

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, struct route_key);
    __type(value, struct route_value);
} routes_map SEC(".maps");

SEC("xdp_router")
int xdp_router_func(struct xdp_md *ctx) {
    return XDP_PASS;

    void *data_end = (void *)(long)ctx->data_end;
    void *data = (void *)(long)ctx->data;
    struct ethhdr *eth = data;
    
    if ((void*)(eth + 1) > data_end)
        return XDP_PASS;
    
    if (eth->h_proto != __constant_htons(ETH_P_IP))
        return XDP_PASS;
    
    struct iphdr *ip = data + sizeof(struct ethhdr);
    if ((void*)(ip + 1) > data_end)
        return XDP_PASS;
    
    struct route_key key = {};
    key.src_ip = ip->saddr;
    key.dst_ip = ip->daddr;
    
    __u16 l4_offset = sizeof(struct ethhdr) + ip->ihl * 4;
    if (l4_offset > data_end - data)
        return XDP_PASS;
    
    if (ip->protocol == IPPROTO_TCP) {
        struct tcphdr *tcp = data + l4_offset;
        if ((void*)(tcp + 1) > data_end)
            return XDP_PASS;
        key.src_port = tcp->source;
        key.dst_port = tcp->dest;
    } else if (ip->protocol == IPPROTO_UDP) {
        struct udphdr *udp = data + l4_offset;
        if ((void*)(udp + 1) > data_end)
            return XDP_PASS;
        key.src_port = udp->source;
        key.dst_port = udp->dest;
    } else {
        return XDP_PASS;
    }
    
    struct route_value *value = bpf_map_lookup_elem(&routes_map, &key);
    if (value) {
        // Replace destination MAC with next hop MAC
        __builtin_memcpy(eth->h_dest, value->next_hop_mac, 6);
        // Set source MAC to interface MAC (could be set if needed)
        // __builtin_memcpy(eth->h_source, ..., 6);
        return XDP_TX;
    }
    
    return XDP_PASS;
}

char _license[] SEC("license") = "GPL";