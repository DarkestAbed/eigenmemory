package wiki

import "testing"

func TestFenceFor(t *testing.T) {
	cases := []struct {
		in   string
		want int // length of the returned fence
	}{
		{"no backticks here", 3},
		{"", 3},
		{"`` short run", 3},   // longest run 2 < 3 -> minimum 3
		{"has ``` fence", 4},  // longest run 3 -> 4
		{"`````` six", 7},     // longest run 6 -> 7
		{"`` ` ```", 4},       // runs 2,1,3 -> longest 3 -> 4
	}
	for _, c := range cases {
		got := FenceFor(c.in)
		if len(got) != c.want {
			t.Errorf("FenceFor(%q) = %q (len %d), want len %d", c.in, got, len(got), c.want)
		}
		// The result must itself be a run of backticks at least 3 long.
		for _, r := range got {
			if r != '`' {
				t.Errorf("FenceFor(%q) = %q, must be only backticks", c.in, got)
				break
			}
		}
	}
}

func TestStripLegacyMdSuffix(t *testing.T) {
	cases := []struct{ in, want string }{
		{"target-md", "target"},
		{"target", "target"},
		{"auth-service-md", "auth-service"},
		{"", ""},
		{"-md", ""},
	}
	for _, c := range cases {
		if got := StripLegacyMdSuffix(c.in); got != c.want {
			t.Errorf("StripLegacyMdSuffix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}