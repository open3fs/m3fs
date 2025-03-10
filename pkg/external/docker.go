package external

// DockerInterface provides interface about docker.
type DockerInterface interface {
	GetContainer(string) string
}

type dockerExternal struct {
	externalBase
}

func (de *dockerExternal) init(em *Manager) {
	de.externalBase.init(em)
	em.Docker = de
}

func (de *dockerExternal) GetContainer(name string) string {
	// TODO: implement docker.GetContainer
	return ""
}

func init() {
	registerNewExternalFunc(func() externalInterface {
		return new(dockerExternal)
	})
}
