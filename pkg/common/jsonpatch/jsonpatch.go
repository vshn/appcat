package jsonpatch

// JSONop is used to define what type of patch should be used
type JSONop string

const (
	// JSONopRemove remove path
	JSONopRemove JSONop = "remove"
	// JSONopAdd add given value to path
	JSONopAdd JSONop = "add"
	// JSONopNone noop
	JSONopNone JSONop = "none"
	// JSONopReplace replace value at the given path
	JSONopReplace JSONop = "replace"
)

// JSONpatch describes a JSON patch that can be used against he k8s API
type JSONpatch struct {
	Op    JSONop `json:"op,omitempty"`
	Path  string `json:"path,omitempty"`
	Value string `json:"value,omitempty"`
}
