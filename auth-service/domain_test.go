package main

import "testing"

func TestDomainInviteSelection(t *testing.T) {
	for _, row := range []struct {
		in, base string
		index    int
		bad      bool
	}{{"94F195C41B5_1", "94F195C41B5", 1, false}, {"94F195C41B5", "94F195C41B5", 0, false}, {"x_0", "", 0, true}, {"x_-1", "", 0, true}, {"x_a", "", 0, true}} {
		base, index, e := domainInviteSelection(row.in)
		if (e != nil) != row.bad || base != row.base || index != row.index {
			t.Fatalf("%s -> %s,%d,%v", row.in, base, index, e)
		}
	}
}
