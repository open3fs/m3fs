package external

// NetInterface provides interface about network.
type NetInterface interface {
	GetRdmaLinks() ([]string, error)
}

type netExternal struct {
	externalBase
}

func (ne *netExternal) init(em *Manager) {
	ne.externalBase.init(em)
	em.Net = ne
}

func (ne *netExternal) GetRdmaLinks() ([]string, error) {
	// TODO: add get list of RDMA links logic
	return nil, nil
}

func init() {
	registerNewExternalFunc(func() externalInterface {
		return new(netExternal)
	})
}
