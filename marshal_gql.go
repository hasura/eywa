package eywa

type GQLMarshaler interface {
	MarshalGQL() string
}

type HasuraEnum string

func (he HasuraEnum) MarshalGQL() string {
	_ = x(he)
	return string(he)
}

func x(q GQLMarshaler) string {
	return "abcd"
}
