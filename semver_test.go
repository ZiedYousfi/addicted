package main

import "testing"

func TestParseSemver(t *testing.T) {
	t.Run("parses plain prefixed prerelease and underscore variants", func(t *testing.T) {
		tests := map[string]Semver{
			"1.2.3":                   {Major: 1, Minor: 2, Patch: 3},
			"v4.5.6":                  {Major: 4, Minor: 5, Patch: 6},
			" 7.8.9 ":                 {Major: 7, Minor: 8, Patch: 9},
			"1_2_3":                   {Major: 1, Minor: 2, Patch: 3},
			"1_2_3_1":                 {Major: 1, Minor: 2, Patch: 3, Revision: 1, HasRevision: true},
			"1.2.3_1":                 {Major: 1, Minor: 2, Patch: 3, Revision: 1, HasRevision: true},
			"1.2.3-beta.1":            {Major: 1, Minor: 2, Patch: 3, PreRelease: []string{"beta", "1"}},
			"1.2.3+build.7":           {Major: 1, Minor: 2, Patch: 3, BuildMetadata: []string{"build", "7"}},
			"1.2.3_1-rc.1+sha.abcdef": {Major: 1, Minor: 2, Patch: 3, Revision: 1, HasRevision: true, PreRelease: []string{"rc", "1"}, BuildMetadata: []string{"sha", "abcdef"}},
		}

		for input, want := range tests {
			got, err := ParseSemver(input)
			if err != nil {
				t.Fatalf("ParseSemver(%q) returned error: %v", input, err)
			}
			if got.String() != want.String() {
				t.Fatalf("ParseSemver(%q) = %+v, want %+v", input, got, want)
			}
		}
	})

	t.Run("rejects invalid versions", func(t *testing.T) {
		inputs := []string{"", "1", "1.2", "1.2.3.4", "1.two.3", "1.-2.3", "01.2.3", "1.2.03", "1.2.3-", "1.2.3+", "1.2.3-beta..1", "1.2.3-01", "1.2.3_", "1_2_3_01"}

		for _, input := range inputs {
			if _, err := ParseSemver(input); err == nil {
				t.Fatalf("ParseSemver(%q) expected error, got nil", input)
			}
		}
	})
}

func TestSemverCompare(t *testing.T) {
	tests := []struct {
		name  string
		left  Semver
		right Semver
		want  int
	}{
		{name: "major", left: Semver{Major: 2, Minor: 0, Patch: 0}, right: Semver{Major: 1, Minor: 9, Patch: 9}, want: 1},
		{name: "minor", left: Semver{Major: 1, Minor: 2, Patch: 0}, right: Semver{Major: 1, Minor: 3, Patch: 0}, want: -1},
		{name: "patch", left: Semver{Major: 1, Minor: 2, Patch: 3}, right: Semver{Major: 1, Minor: 2, Patch: 2}, want: 1},
		{name: "release greater than prerelease", left: Semver{Major: 1, Minor: 2, Patch: 3}, right: Semver{Major: 1, Minor: 2, Patch: 3, PreRelease: []string{"rc", "1"}}, want: 1},
		{name: "revision greater than no revision", left: Semver{Major: 1, Minor: 2, Patch: 3, Revision: 1, HasRevision: true}, right: Semver{Major: 1, Minor: 2, Patch: 3}, want: 1},
		{name: "revision numeric", left: Semver{Major: 1, Minor: 2, Patch: 3, Revision: 1, HasRevision: true}, right: Semver{Major: 1, Minor: 2, Patch: 3, Revision: 2, HasRevision: true}, want: -1},
		{name: "equal ignores build metadata", left: Semver{Major: 1, Minor: 2, Patch: 3, BuildMetadata: []string{"a"}}, right: Semver{Major: 1, Minor: 2, Patch: 3, BuildMetadata: []string{"b"}}, want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.left.Compare(test.right)
			if got != test.want {
				t.Fatalf("Compare() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestSemverHelpers(t *testing.T) {
	left := Semver{Major: 1, Minor: 2, Patch: 3}
	right := Semver{Major: 1, Minor: 2, Patch: 4}

	if !left.LessThan(right) {
		t.Fatal("expected LessThan to report true")
	}
	if left.Equal(right) {
		t.Fatal("expected Equal to report false")
	}
	if !left.Equal(Semver{Major: 1, Minor: 2, Patch: 3}) {
		t.Fatal("expected Equal to report true")
	}
	if got := (Semver{Major: 1, Minor: 2, Patch: 3, Revision: 1, HasRevision: true, PreRelease: []string{"beta", "1"}, BuildMetadata: []string{"build", "7"}}).String(); got != "1.2.3_1-beta.1+build.7" {
		t.Fatalf("String() = %q, want %q", got, "1.2.3_1-beta.1+build.7")
	}
	if !left.IsPatchUpdate(right) {
		t.Fatal("expected IsPatchUpdate to report true")
	}
	if left.IsMinorUpdate(right) {
		t.Fatal("expected IsMinorUpdate to report false")
	}
	if left.IsMajorUpdate(right) {
		t.Fatal("expected IsMajorUpdate to report false")
	}
	if !(Semver{Major: 1, Minor: 2, Patch: 3}).IsRevisionUpdate(Semver{Major: 1, Minor: 2, Patch: 3, Revision: 1, HasRevision: true}) {
		t.Fatal("expected IsRevisionUpdate to report true")
	}
	if diff := left.Diff(right); diff != "1.2.3 -> 1.2.4 (patch)" {
		t.Fatalf("Diff() = %q, want %q", diff, "1.2.3 -> 1.2.4 (patch)")
	}
}

func TestCompareSemver(t *testing.T) {
	got, err := CompareSemver("1.10.0", "1.2.9")
	if err != nil {
		t.Fatalf("CompareSemver returned error: %v", err)
	}
	if got != 1 {
		t.Fatalf("CompareSemver() = %d, want 1", got)
	}

	underscoreCompare, err := CompareSemver("1.2.3_1", "1.2.4")
	if err != nil {
		t.Fatalf("CompareSemver underscore returned error: %v", err)
	}
	if underscoreCompare != -1 {
		t.Fatalf("CompareSemver underscore = %d, want -1", underscoreCompare)
	}

	if _, err := CompareSemver("bad", "1.0.0"); err == nil {
		t.Fatal("expected invalid left semver to return an error")
	}
}

func TestCompareSemverChange(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
		want  SemverChange
	}{
		{name: "none", left: "1.2.3", right: "1.2.3", want: SemverChangeNone},
		{name: "patch", left: "1.2.3", right: "1.2.4", want: SemverChangePatch},
		{name: "minor", left: "1.2.3", right: "1.3.0", want: SemverChangeMinor},
		{name: "major", left: "1.2.3", right: "2.0.0", want: SemverChangeMajor},
		{name: "prerelease", left: "1.2.3-rc.1", right: "1.2.3", want: SemverChangePrerelease},
		{name: "revision", left: "1.2.3", right: "1.2.3_1", want: SemverChangeRevision},
		{name: "downgrade", left: "2.0.0", right: "1.9.9", want: SemverChangeDowngrade},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := CompareSemverChange(test.left, test.right)
			if err != nil {
				t.Fatalf("CompareSemverChange returned error: %v", err)
			}
			if got != test.want {
				t.Fatalf("CompareSemverChange() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestStringUpdateHelpers(t *testing.T) {
	isPatch, err := IsPatchSemverUpdate("1.2.3", "1.2.4")
	if err != nil || !isPatch {
		t.Fatalf("expected patch helper to return true, got %v, %v", isPatch, err)
	}

	isMinor, err := IsMinorSemverUpdate("1.2.3", "1.3.0")
	if err != nil || !isMinor {
		t.Fatalf("expected minor helper to return true, got %v, %v", isMinor, err)
	}

	isMajor, err := IsMajorSemverUpdate("1.2.3", "2.0.0")
	if err != nil || !isMajor {
		t.Fatalf("expected major helper to return true, got %v, %v", isMajor, err)
	}
}
