package vc

import "testing"

func TestClientPath(t *testing.T) {
	tests := []struct {
		Path string
		Test string
		Want string
	}{
		{"", "test", "/test"},
		{"", ".", "/"},
		{"foo", "bar", "/foo/bar"},
		{"/", "foo", "/foo"},
		{"/", "foo/", "/foo"},
	}

	c := new(Client)
	for _, test := range tests {
		c.SetPath(test.Path)
		if path := c.abspath(test.Test); path != test.Want {
			t.Fatalf("abspath(%q) in %q; expected %q, got %q", test.Test, test.Path, test.Want, path)
		}
	}
}
