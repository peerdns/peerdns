package ebpf

import (
	"github.com/peerdns/peerdns/pkg/messages"
	"net"
	"testing"

	"github.com/peerdns/peerdns/pkg/config"
)

func TestEBPFManager(t *testing.T) {
	// Define test eBPF config
	testConfig := config.EBPF{
		InterfaceName: "lo", // Use loopback interface for testing
		Programs: []config.EBPFProgram{
			{
				Name:       "xdp_test_pass_func",
				Section:    "xdp_test_pass_func",
				IsTest:     true,
				ObjectPath: "obj/ebpf_test_program.o", // Not used since embedded
			},
		},
	}

	// Create Manager
	manager, err := NewManager(testConfig)
	if err != nil {
		t.Fatalf("Failed to create EBPFManager: %v", err)
	}
	defer manager.Close()

	// Define a route entry
	routeEntry := &messages.RouteEntry{
		SrcIP:      net.ParseIP("192.168.1.100"),
		DstIP:      net.ParseIP("192.168.1.200"),
		SrcPort:    12345,
		DstPort:    80,
		NextHopMAC: net.HardwareAddr{0x00, 0x0c, 0x29, 0xab, 0xcd, 0xef},
	}

	// Add route
	err = manager.AddRoute(routeEntry)
	if err != nil {
		t.Fatalf("Failed to add route: %v", err)
	}

	// Verify that the route exists in the map
	key := constructKey(routeEntry)
	var value RouteValue
	err = manager.Maps["routes_map"].Lookup(&key, &value)
	if err != nil {
		t.Fatalf("Failed to lookup route: %v", err)
	}

	// Compare the value
	expectedMAC := [6]byte{0x00, 0x0c, 0x29, 0xab, 0xcd, 0xef}
	if value.NextHopMAC != expectedMAC {
		t.Fatalf("Unexpected NextHopMAC in map. Expected %v, got %v", expectedMAC, value.NextHopMAC)
	}

	// Delete the route
	err = manager.DeleteRoute(routeEntry)
	if err != nil {
		t.Fatalf("Failed to delete route: %v", err)
	}

	// Verify that the route is deleted
	err = manager.Maps["routes_map"].Lookup(&key, &value)
	if err == nil {
		t.Fatalf("Expected route to be deleted, but found in map")
	}
}
