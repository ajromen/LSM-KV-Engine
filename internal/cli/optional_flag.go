package cli

type OptionalBool struct {
	val bool
	set bool
}

func (o *OptionalBool) Get() *bool {
	if !o.set {
		return nil
	}
	return &o.val
}

type OptionalString struct {
	val string
	set bool
}

func (o *OptionalString) Get() *string {
	if !o.set {
		return nil
	}
	return &o.val
}

type OptionalInt struct {
	val int
	set bool
}

func (o *OptionalInt) Get() *int {
	if !o.set {
		return nil
	}
	return &o.val
}
