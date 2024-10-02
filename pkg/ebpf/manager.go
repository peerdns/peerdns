package ebpf

import (
	"bytes"
	"fmt"
	"github.com/peerdns/peerdns/pkg/messages"
	"net"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/peerdns/peerdns/pkg/config"
)

// Embed eBPF programs.
//
//go:embed "../../c/obj/ebpf_program.o"
var ebpfProgram []byte

//go:embed "../../c/obj/ebpf_test_program.o"
var ebpfTestProgram []byte

// Manager handles loading, attaching, and managing eBPF programs and maps.
type Manager struct {
	Programs      map[string]*ebpf.Program
	Maps          map[string]*ebpf.Map
	Links         map[string]link.Link
	mu            sync.Mutex
	interfaceName string
}

// NewManager creates a new EBPF Manager based on the configuration.
func NewManager(cfg config.EBPF) (*Manager, error) {
	manager := &Manager{
		Programs:      make(map[string]*ebpf.Program),
		Maps:          make(map[string]*ebpf.Map),
		Links:         make(map[string]link.Link),
		interfaceName: cfg.InterfaceName,
	}

	for _, progCfg := range cfg.Programs {
		var progBytes []byte
		if progCfg.IsTest {
			progBytes = ebpfTestProgram
		} else {
			progBytes = ebpfProgram
		}

		spec, err := ebpf.LoadCollectionSpecFromReader(bytes.NewReader(progBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to load eBPF collection spec for program %s: %w", progCfg.Name, err)
		}

		// Load the eBPF collection
		coll, err := ebpf.NewCollection(spec)
		if err != nil {
			return nil, fmt.Errorf("failed to create eBPF collection for program %s: %w", progCfg.Name, err)
		}

		program := coll.Programs[progCfg.Section]
		if program == nil {
			return nil, fmt.Errorf("failed to find program section %s for program %s", progCfg.Section, progCfg.Name)
		}

		routesMap := coll.Maps["routes_map"]

		// Attach the XDP program to the specified network interface
		ifaceIndex, err := interfaceByName(manager.interfaceName)
		if err != nil {
			return nil, fmt.Errorf("failed to get interface index: %w", err)
		}

		xdpLink, err := link.AttachXDP(link.XDPOptions{
			Program:   program,
			Interface: ifaceIndex,
			Flags:     link.XDPGenericMode, // Use generic mode if hardware offload is not supported
		})
		if err != nil {
			// Close the program and map on failure
			program.Close()
			if routesMap != nil {
				routesMap.Close()
			}
			return nil, fmt.Errorf("failed to attach XDP program %s: %w", progCfg.Name, err)
		}

		// Store in manager
		manager.Programs[progCfg.Name] = program
		if routesMap != nil {
			manager.Maps["routes_map"] = routesMap
		}
		manager.Links[progCfg.Name] = xdpLink
	}

	return manager, nil
}

// Close detaches all eBPF programs and closes resources.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, link := range m.Links {
		if link != nil {
			link.Close()
		}
		delete(m.Links, name)
	}
	for name, program := range m.Programs {
		if program != nil {
			program.Close()
		}
		delete(m.Programs, name)
	}
	for name, bpfMap := range m.Maps {
		if bpfMap != nil {
			bpfMap.Close()
		}
		delete(m.Maps, name)
	}
}

// AddRoute adds a routing entry to the routes_map.
func (m *Manager) AddRoute(entry *messages.RouteEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Maps["routes_map"] == nil {
		return fmt.Errorf("routes_map is not initialized")
	}

	key := constructKey(entry)
	value := constructValue(entry)

	// Put the entry into the map
	err := m.Maps["routes_map"].Put(&key, &value)
	if err != nil {
		return fmt.Errorf("failed to add route to eBPF map: %w", err)
	}
	return nil
}

// DeleteRoute deletes a routing entry from the routes_map.
func (m *Manager) DeleteRoute(entry *messages.RouteEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Maps["routes_map"] == nil {
		return fmt.Errorf("routes_map is not initialized")
	}

	key := constructKey(entry)

	err := m.Maps["routes_map"].Delete(&key)
	if err != nil {
		return fmt.Errorf("failed to delete route from eBPF map: %w", err)
	}
	return nil
}

// LoadDefaults loads default eBPF programs.
// This function can be expanded to load additional default programs as needed.
func (m *Manager) LoadDefaults(defaultPrograms []config.EBPFProgram) error {
	for _, progCfg := range defaultPrograms {
		// Prevent re-loading already loaded programs
		if _, exists := m.Programs[progCfg.Name]; exists {
			continue
		}

		var progBytes []byte
		if progCfg.IsTest {
			progBytes = ebpfTestProgram
		} else {
			progBytes = ebpfProgram
		}

		spec, err := ebpf.LoadCollectionSpecFromReader(bytes.NewReader(progBytes))
		if err != nil {
			return fmt.Errorf("failed to load eBPF collection spec for program %s: %w", progCfg.Name, err)
		}

		// Load the eBPF collection
		coll, err := ebpf.NewCollection(spec)
		if err != nil {
			return fmt.Errorf("failed to create eBPF collection for program %s: %w", progCfg.Name, err)
		}

		program := coll.Programs[progCfg.Section]
		if program == nil {
			return fmt.Errorf("failed to find program section %s for program %s", progCfg.Section, progCfg.Name)
		}

		routesMap := coll.Maps["routes_map"]

		// Attach the XDP program to the specified network interface
		ifaceIndex, err := interfaceByName(m.interfaceName)
		if err != nil {
			return fmt.Errorf("failed to get interface index: %w", err)
		}

		xdpLink, err := link.AttachXDP(link.XDPOptions{
			Program:   program,
			Interface: ifaceIndex,
			Flags:     link.XDPGenericMode, // Use generic mode if hardware offload is not supported
		})
		if err != nil {
			// Close the program and map on failure
			program.Close()
			if routesMap != nil {
				routesMap.Close()
			}
			return fmt.Errorf("failed to attach XDP program %s: %w", progCfg.Name, err)
		}

		// Store in manager
		m.Programs[progCfg.Name] = program
		if routesMap != nil {
			m.Maps["routes_map"] = routesMap
		}
		m.Links[progCfg.Name] = xdpLink
	}

	return nil
}

// interfaceByName retrieves the interface index by name.
func interfaceByName(name string) (int, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return 0, err
	}
	return iface.Index, nil
}
