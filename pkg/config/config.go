package config

import "time"

// NetworkType is the type of network definition
type NetworkType string

// defines network types
const (
	NetworkTypeRDMA NetworkType = "RDMA"
	NetworkTypeRXE  NetworkType = "RXE"
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
	Host          string
	Port          int
	Username      string
	Password      *string  `yaml:",omitempty"`
	RDMAAddresses []string `yaml:"rdmaAddresses,omitempty"`
}

// Fdb is the fdb config definition
type Fdb struct {
	Nodes              []string
	Port               int
	DataDir            string
	WaitClusterTimeout time.Duration
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
	Name        string
	NetworkType NetworkType
	Nodes       []Node
	Services    Services
	Registry    Registry
}
