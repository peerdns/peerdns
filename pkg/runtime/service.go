package runtime

type Service interface {
	Type() ServiceType
	Start() error
	Stop() error
}
