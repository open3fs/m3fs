package external

// DiskInterface provides interface about disk.
type DiskInterface interface {
	GetNvmeDisks() ([]string, error)
}

type diskExternal struct {
	externalBase
}

func (de *diskExternal) init(em *Manager) {
	de.externalBase.init(em)
	em.Disk = de
}

func (de *diskExternal) GetNvmeDisks() ([]string, error) {
	// TODO: implement GetNvmeDisks
	return nil, nil
}

func init() {
	registerNewExternalFunc(func() externalInterface {
		return new(diskExternal)
	})
}
