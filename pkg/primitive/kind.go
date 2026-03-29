package primitive

// Kind identifies a node in the workflow pipeline.
type Kind string

const (
	Call   Kind = "call"
	Set    Kind = "set"
	Filter Kind = "filter"
	Tap    Kind = "tap"
)

func (k Kind) Valid() bool {
	switch k {
	case Call, Set, Filter, Tap:
		return true
	default:
		return false
	}
}
