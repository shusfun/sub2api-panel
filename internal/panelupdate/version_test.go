package panelupdate

import "testing"

func TestProductionReleaseNumbers(t *testing.T) {
	tests := []struct {
		name, previous, next, tag, approved string
		valid                               bool
	}{
		{"first", "", "0.1.0001", "v0.1.1", "", true},
		{"wrong first", "", "0.1.0002", "v0.1.2", "", false},
		{"regular", "0.1.0001", "0.1.0002", "v0.1.2", "", true},
		{"carry within patch", "0.1.0009", "0.1.0010", "v0.1.10", "", true},
		{"skip even with approval", "0.1.0001", "0.1.0003", "v0.1.3", "0.1.0003", false},
		{"minor without approval", "0.1.0001", "0.2.0001", "v0.2.1", "", false},
		{"major without approval", "0.1.0001", "1.1.0001", "v1.1.1", "", false},
		{"different approval", "0.1.0001", "1.0.0001", "v1.0.1", "1.1.0001", false},
		{"approved change", "0.1.9999", "1.0.0001", "v1.0.1", "1.0.0001", true},
		{"no automatic rollover", "0.1.9999", "0.2.0001", "v0.2.1", "", false},
		{"tag drift", "0.1.0001", "0.1.0002", "v1.1.2", "", false},
		{"padded tag", "0.1.0001", "0.1.0002", "v0.1.0002", "", false},
		{"maximum", "9.99.9998", "9.99.9999", "v9.99.9999", "", true},
		{"patch overflow", "9.99.9999", "9.99.10000", "v9.99.10000", "", false},
		{"major overflow", "9.99.9999", "10.0.0001", "v10.0.1", "10.0.0001", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateProductionRelease(test.previous, test.next, test.tag, test.approved)
			if (err == nil) != test.valid {
				t.Fatalf("validation = %v, want valid = %v", err, test.valid)
			}
		})
	}
	for tag, want := range map[string]string{"v0.1.1": "0.1.0001", "v0.1.10": "0.1.0010", "v0.1.0-rc.3": "0.1.0-rc.3"} {
		got, ok := ProductForTag(tag)
		if !ok || got != want {
			t.Fatalf("%s mapped to %q, want %q", tag, got, want)
		}
	}
	if Compare("0.1.0010", "0.1.0009") <= 0 || Compare("0.1.0001", "0.1.0-rc.3") <= 0 {
		t.Fatal("comparison did not follow the version segments")
	}
}

func TestReleaseTagCannotBypassVersionApprovalThroughRC(t *testing.T) {
	rc := Manifest{Version: "1.0.0-rc.1", Tag: "v1.0.0-rc.1"}
	if err := validateAgainstPublished(nil, rc, ""); err == nil {
		t.Fatal("RC tag changed release series without approval")
	}
	if err := validateAgainstPublished(nil, rc, rc.Version); err != nil {
		t.Fatal(err)
	}
	rc = Manifest{Version: "0.1.0-rc.4", Tag: "v0.1.0-rc.4"}
	if err := validateAgainstPublished(nil, rc, ""); err != nil {
		t.Fatal(err)
	}
	if err := validateAgainstPublished([]release{{Tag: rc.Tag, Prerelease: true}}, rc, ""); err == nil {
		t.Fatal("existing RC tag was accepted")
	}
}
