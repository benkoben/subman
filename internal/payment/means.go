package payment

type Means interface {
	IsActive() bool
	Cancel()
}
