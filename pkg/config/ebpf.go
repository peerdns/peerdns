package config

// EBPF holds the configuration for eBPF programs.
type EBPF struct {
	InterfaceName string        `yaml:"interface_name"`
	Programs      []EBPFProgram `yaml:"programs"`
}

// EBPFProgram defines configuration for an eBPF program.
type EBPFProgram struct {
	Name       string `yaml:"name"`
	Section    string `yaml:"section"`
	IsTest     bool   `yaml:"is_test"`
	ObjectPath string `yaml:"object_path"` // Path to the compiled eBPF object file
}
