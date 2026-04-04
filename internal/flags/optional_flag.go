package flags

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

type OptionalUInt16 struct {
	val uint16
	set bool
}

func (o *OptionalUInt16) Get() *uint16 {
	if !o.set {
		return nil
	}
	return &o.val
}

type OptionalUInt32 struct {
	val uint32
	set bool
}

func (o *OptionalUInt32) Get() *uint32 {
	if !o.set {
		return nil
	}
	return &o.val
}

type OptionalUInt64 struct {
	val uint64
	set bool
}

func (o *OptionalUInt64) Get() *uint64 {
	if !o.set {
		return nil
	}
	return &o.val
}
