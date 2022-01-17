package api_test

import (
	"testing"

	"github.com/lestrrat-go/fsnotify/api"
	"github.com/stretchr/testify/assert"
)

func TestOp(t *testing.T) {
	t.Run("Stringification", func(t *testing.T) {
		testcases := []struct {
			Op       api.Op
			Expected string
		}{
			{
				Op:       api.OpCreate,
				Expected: "CREATE",
			},
			{
				Op:       api.OpWrite,
				Expected: "WRITE",
			},
			{
				Op:       api.OpRemove,
				Expected: "REMOVE",
			},
			{
				Op:       api.OpRename,
				Expected: "RENAME",
			},
			{
				Op:       api.OpChmod,
				Expected: "CHMOD",
			},
			{
				Op:       api.Op(0),
				Expected: "INVALID OP",
			},
		}

		for _, tc := range testcases {
			tc := tc
			t.Run(tc.Op.String(), func(t *testing.T) {
				if !assert.Equal(t, tc.Expected, tc.Op.String(), `stringification should match`) {
					return
				}
			})
		}
	})
}

func TestOpMask(t *testing.T) {
	t.Run("Manipulation", func(t *testing.T) {
		var op = api.OpCreate
		var mask api.OpMask

		if !assert.False(t, mask.IsSet(op), `mask.IsSet should be false`) {
			return
		}
		mask.Set(op)
		if !assert.True(t, mask.IsSet(op), `mask.IsSet should be true`) {
			return
		}
		mask.Unset(op)
		if !assert.False(t, mask.IsSet(op), `mask.IsSet should be false`) {
			return
		}
	})
	t.Run("Stringification", func(t *testing.T) {
		var mask api.OpMask
		if !assert.Equal(t, "", mask.String(), `empty mask should show empty string`) {
			return
		}

		mask.Set(api.OpCreate)
		if !assert.Equal(t, api.OpCreate.String(), mask.String(), `when single op is set, it should be equal to op.String()`) {
			return
		}

		mask.Set(api.OpWrite)
		if !assert.Equal(t, "CREATE|WRITE", mask.String(), `multiple ops should be concatenated by |`) {
			return
		}
	})
}
