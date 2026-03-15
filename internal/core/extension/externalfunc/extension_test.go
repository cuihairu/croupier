package externalfunc

import "testing"

func TestIsExternalPlatformExtensionID(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{id: "official.external-platform", want: true},
		{id: "foo.external", want: true},
		{id: "official.analytics", want: false},
	}
	for _, tc := range cases {
		if got := IsExternalPlatformExtensionID(tc.id); got != tc.want {
			t.Fatalf("id=%s expected %v got %v", tc.id, tc.want, got)
		}
	}
}
