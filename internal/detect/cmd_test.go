package detect

import "testing"

func TestArgRootAcceptsPatternSuffix(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, "."},
		{[]string{"."}, "."},
		{[]string{"./..."}, "."},
		{[]string{"..."}, "."},
		{[]string{"internal/..."}, "internal"},
		{[]string{"./internal/..."}, "./internal"},
		{[]string{"internal"}, "internal"},
	}
	for _, c := range cases {
		if got := argRoot(c.args); got != c.want {
			t.Errorf("argRoot(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}
