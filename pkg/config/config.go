package config

// NetworkType is the type of network definition
type NetworkType string

// defines network types
const (
	NetworkTypeRoce NetworkType = "RoCE"
	NetworkTypeRXE  NetworkType = "rxe"
)

// DiskType is the type of disk definition
type DiskType string

// defines disk types
const (
	DiskTypeDirectory DiskType = "directory"
	DiskTypeNvme      DiskType = "NVMe"
)

// Node is the node config definition
type Node struct {
	Name          string
	Address       string
	Username      string
	Password      string
	RDMAAddresses []string `yaml:"rdmaAddresses"`
}

// Fdb is the fdb config definition
type Fdb struct {
	Nodes   []string
	DataDir string
}

// Clickhouse is the clickhouse config definition
type Clickhouse struct {
	Nodes   []string
	DataDir string
}

// Mgmtd is the 3fs mgmtd service config definition
type Mgmtd struct {
	Nodes []string
}

// Meta is the 3fs meta service config definition
type Meta struct {
	Nodes []string
}

// Storage is the 3fs storage config definition
type Storage struct {
	Nodes    []string
	DiskType DiskType
}

// Client is the 3fs client config definition
type Client struct {
	Nodes []string
}

// Services is the services config definition
type Services struct {
	Fdb        Fdb
	Clickhouse Clickhouse
	Mgmtd      Mgmtd
	Meta       Meta
	Storage    Storage
	Client     Client
}

// Registry is the service contaner image registry config definition
type Registry struct {
	CustomRegistry string
}

// Config is the 3fs cluster config definition
type Config struct {
	NetworkType NetworkType
	Nodes       []Node
	Services    Services
	Registry    Registry
}
