package monitoring

import "testing"

func TestEffectiveFinalUsesCurrentStatusOnly(t *testing.T) {
	for _, testCase := range []struct{ status, want string }{
		{statusPublished, finalAccepted},
		{statusDeclined, finalRejected},
		{statusRejected, finalRejected},
		{statusWithdrawn, finalWithdrawn},
		{statusCopyediting, ""},
		{"Review", ""},
		{"Pending", ""},
	} {
		if got := effectiveFinal(testCase.status); got != testCase.want {
			t.Errorf("effectiveFinal(%q) = %q, want %q", testCase.status, got, testCase.want)
		}
	}
}
