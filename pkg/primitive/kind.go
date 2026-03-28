package primitive

// Kind identifies a node in the workflow pipeline.
type Kind string

const (
	Exec   Kind = "exec"
	Set    Kind = "set"
	Filter Kind = "filter"
	Tap    Kind = "tap"
)

func (k Kind) Valid() bool {
	switch k {
	case Exec, Set, Filter, Tap:
		return true
	default:
		return false
	}
}
