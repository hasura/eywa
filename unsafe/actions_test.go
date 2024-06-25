// nolint
package unsafe

import (
	"testing"

	"github.com/imperfect-fourth/eywa"
	"github.com/stretchr/testify/assert"
)

type testAction struct {
	Name string `json:"name"`
	ID   *int   `json:"id,omitempty"`
}

func (a testAction) ModelName() string {
	return "test_action"
}

func TestActionQuery(t *testing.T) {
	q := Action[testAction]().QueryArgs(
		map[string]interface{}{
			"arg1": 4,
			"arg2": "value",
			"arg3": eywa.HasuraEnum("enumvalue"),
		},
	).Select("name")

	expected := `query run_test_action {
test_action(arg1: 4, arg2: "value", arg3: enumvalue) {
name
}
}`
	assert.Equal(t, expected, q.Query())
}
