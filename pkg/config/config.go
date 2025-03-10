package config

// NetworkType is the type of network defination
type NetworkType string

// defines network types
const (
	NetworkTypeRoce NetworkType = "RoCE"
	NetworkTypeRXE  NetworkType = "rxe"
)

// DiskType is the type of disk defination
type DiskType string

// defines disk types
const (
	DiskTypeDirectory DiskType = "directory"
	DiskTypeNvme      DiskType = "NVMe"
)

// Node is the node config defination
type Node struct {
	Name          string
	Address       string
	Username      string
	Password      string
	RDMAAddresses []string `yaml:"rdmaAddresses"`
}

// Fdb is the fdb config defination
type Fdb struct {
	Nodes   []string
	DataDir string
}

// Clickhouse is the clickhouse config defination
type Clickhouse struct {
	Nodes   []string
	DataDir string
}

// Mgmtd is the 3fs mgmtd service config defination
type Mgmtd struct {
	Nodes []string
}

// Meta is the 3fs meta service config defination
type Meta struct {
	Nodes []string
}

// Storage is the 3fs storage config defination
type Storage struct {
	Nodes    []string
	DiskType DiskType
}

// Client is the 3fs client config defination
type Client struct {
	Nodes []string
}

// Services is the services config defination
type Services struct {
	Fdb        Fdb
	Clickhouse Clickhouse
	Mgmtd      Mgmtd
	Meta       Meta
	Storage    Storage
	Client     Client
}

// Registry is the service contaner image registry config defination
type Registry struct {
	CustomRegistry string
}

// Config is the 3fs cluster config defination
type Config struct {
	NetworkType NetworkType
	Nodes       []Node
	Services    Services
	Registry    Registry
}
