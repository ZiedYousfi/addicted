package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Semver struct {
	Major         int
	Minor         int
	Patch         int
	Revision      int
	HasRevision   bool
	PreRelease    []string
	BuildMetadata []string
}

type SemverChange string

const (
	SemverChangeInvalid    SemverChange = "invalid"
	SemverChangeDowngrade  SemverChange = "downgrade"
	SemverChangeNone       SemverChange = "none"
	SemverChangePrerelease SemverChange = "prerelease"
	SemverChangeRevision   SemverChange = "revision"
	SemverChangePatch      SemverChange = "patch"
	SemverChangeMinor      SemverChange = "minor"
	SemverChangeMajor      SemverChange = "major"
)

func ParseSemver(value string) (Semver, error) {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "v")
	if trimmed == "" {
		return Semver{}, fmt.Errorf("invalid semver %q: empty value", value)
	}

	coreAndPrerelease := trimmed
	buildMetadata := []string(nil)
	if corePart, buildPart, ok := strings.Cut(trimmed, "+"); ok {
		coreAndPrerelease = corePart
		parsedBuildMetadata, err := parseSemverIdentifiers(buildPart, "build metadata", false)
		if err != nil {
			return Semver{}, err
		}
		buildMetadata = parsedBuildMetadata
	}

	coreVersion := coreAndPrerelease
	preRelease := []string(nil)
	if corePart, prereleasePart, ok := strings.Cut(coreAndPrerelease, "-"); ok {
		coreVersion = corePart
		parsedPreRelease, err := parseSemverIdentifiers(prereleasePart, "prerelease", true)
		if err != nil {
			return Semver{}, err
		}
		preRelease = parsedPreRelease
	}

	major, minor, patch, revision, hasRevision, err := parseSemverCore(coreVersion, value)
	if err != nil {
		return Semver{}, err
	}

	return Semver{
		Major:         major,
		Minor:         minor,
		Patch:         patch,
		Revision:      revision,
		HasRevision:   hasRevision,
		PreRelease:    preRelease,
		BuildMetadata: buildMetadata,
	}, nil
}

func parseSemverCore(coreVersion string, originalValue string) (int, int, int, int, bool, error) {
	var parts []string
	hasRevision := false
	revisionPart := ""

	if strings.Contains(coreVersion, ".") {
		parts = strings.Split(coreVersion, ".")
		if len(parts) != 3 {
			return 0, 0, 0, 0, false, fmt.Errorf("invalid semver %q: expected major.minor.patch", originalValue)
		}

		patchPart := parts[2]
		if patchCore, patchRevision, ok := strings.Cut(patchPart, "_"); ok {
			parts[2] = patchCore
			revisionPart = patchRevision
			hasRevision = true
		}
	} else {
		parts = strings.Split(coreVersion, "_")
		switch len(parts) {
		case 3:
		case 4:
			revisionPart = parts[3]
			hasRevision = true
		default:
			return 0, 0, 0, 0, false, fmt.Errorf("invalid semver %q: expected major.minor.patch", originalValue)
		}
	}

	major, err := parseSemverPart(parts[0], "major")
	if err != nil {
		return 0, 0, 0, 0, false, err
	}

	minor, err := parseSemverPart(parts[1], "minor")
	if err != nil {
		return 0, 0, 0, 0, false, err
	}

	patch, err := parseSemverPart(parts[2], "patch")
	if err != nil {
		return 0, 0, 0, 0, false, err
	}

	revision := 0
	if hasRevision {
		revision, err = parseSemverPart(revisionPart, "revision")
		if err != nil {
			return 0, 0, 0, 0, false, err
		}
	}

	return major, minor, patch, revision, hasRevision, nil
}

func parseSemverPart(value string, name string) (int, error) {
	if value == "" {
		return 0, fmt.Errorf("invalid semver %s: empty", name)
	}
	if len(value) > 1 && value[0] == '0' {
		return 0, fmt.Errorf("invalid semver %s %q: leading zero", name, value)
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid semver %s %q: %w", name, value, err)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("invalid semver %s %q: must be non-negative", name, value)
	}

	return parsed, nil
}

func parseSemverIdentifiers(value string, label string, rejectLeadingZeroNumbers bool) ([]string, error) {
	if value == "" {
		return nil, fmt.Errorf("invalid semver %s: empty", label)
	}

	parts := strings.Split(value, ".")
	identifiers := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("invalid semver %s: empty identifier", label)
		}
		for _, r := range part {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' {
				return nil, fmt.Errorf("invalid semver %s %q: contains %q", label, part, r)
			}
		}
		if rejectLeadingZeroNumbers && isNumericIdentifier(part) && len(part) > 1 && part[0] == '0' {
			return nil, fmt.Errorf("invalid semver %s %q: leading zero", label, part)
		}
		identifiers = append(identifiers, part)
	}

	return identifiers, nil
}

func (s Semver) Compare(other Semver) int {
	if s.Major != other.Major {
		return compareInts(s.Major, other.Major)
	}
	if s.Minor != other.Minor {
		return compareInts(s.Minor, other.Minor)
	}
	if s.Patch != other.Patch {
		return compareInts(s.Patch, other.Patch)
	}

	preReleaseComparison := comparePreRelease(s.PreRelease, other.PreRelease)
	if preReleaseComparison != 0 {
		return preReleaseComparison
	}

	return compareRevision(s, other)
}

func (s Semver) LessThan(other Semver) bool {
	return s.Compare(other) < 0
}

func (s Semver) Equal(other Semver) bool {
	return s.Compare(other) == 0
}

func (s Semver) ChangeType(other Semver) SemverChange {
	comparison := s.Compare(other)
	switch {
	case comparison > 0:
		return SemverChangeDowngrade
	case comparison == 0:
		return SemverChangeNone
	case s.Major != other.Major:
		return SemverChangeMajor
	case s.Minor != other.Minor:
		return SemverChangeMinor
	case s.Patch != other.Patch:
		return SemverChangePatch
	case s.HasRevision != other.HasRevision || s.Revision != other.Revision:
		return SemverChangeRevision
	default:
		return SemverChangePrerelease
	}
}

func (s Semver) IsPatchUpdate(other Semver) bool {
	return s.ChangeType(other) == SemverChangePatch
}

func (s Semver) IsMinorUpdate(other Semver) bool {
	return s.ChangeType(other) == SemverChangeMinor
}

func (s Semver) IsMajorUpdate(other Semver) bool {
	return s.ChangeType(other) == SemverChangeMajor
}

func (s Semver) IsRevisionUpdate(other Semver) bool {
	return s.ChangeType(other) == SemverChangeRevision
}

func (s Semver) String() string {
	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("%d.%d.%d", s.Major, s.Minor, s.Patch))
	if s.HasRevision {
		builder.WriteByte('_')
		builder.WriteString(strconv.Itoa(s.Revision))
	}
	if len(s.PreRelease) > 0 {
		builder.WriteByte('-')
		builder.WriteString(strings.Join(s.PreRelease, "."))
	}
	if len(s.BuildMetadata) > 0 {
		builder.WriteByte('+')
		builder.WriteString(strings.Join(s.BuildMetadata, "."))
	}
	return builder.String()
}

func CompareSemver(left string, right string) (int, error) {
	leftSemver, err := ParseSemver(left)
	if err != nil {
		return 0, err
	}

	rightSemver, err := ParseSemver(right)
	if err != nil {
		return 0, err
	}

	return leftSemver.Compare(rightSemver), nil
}

func CompareSemverChange(left string, right string) (SemverChange, error) {
	leftSemver, err := ParseSemver(left)
	if err != nil {
		return SemverChangeInvalid, err
	}

	rightSemver, err := ParseSemver(right)
	if err != nil {
		return SemverChangeInvalid, err
	}

	return leftSemver.ChangeType(rightSemver), nil
}

func IsPatchSemverUpdate(left string, right string) (bool, error) {
	return isSemverUpdateType(left, right, SemverChangePatch)
}

func IsMinorSemverUpdate(left string, right string) (bool, error) {
	return isSemverUpdateType(left, right, SemverChangeMinor)
}

func IsMajorSemverUpdate(left string, right string) (bool, error) {
	return isSemverUpdateType(left, right, SemverChangeMajor)
}

func isSemverUpdateType(left string, right string, changeType SemverChange) (bool, error) {
	actualChangeType, err := CompareSemverChange(left, right)
	if err != nil {
		return false, err
	}
	return actualChangeType == changeType, nil
}

func compareInts(left int, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func comparePreRelease(left []string, right []string) int {
	if len(left) == 0 && len(right) == 0 {
		return 0
	}
	if len(left) == 0 {
		return 1
	}
	if len(right) == 0 {
		return -1
	}

	maxLen := len(left)
	if len(right) > maxLen {
		maxLen = len(right)
	}

	for i := 0; i < maxLen; i++ {
		if i >= len(left) {
			return -1
		}
		if i >= len(right) {
			return 1
		}

		cmp := compareIdentifier(left[i], right[i])
		if cmp != 0 {
			return cmp
		}
	}

	return 0
}

func compareRevision(left Semver, right Semver) int {
	if !left.HasRevision && !right.HasRevision {
		return 0
	}
	if !left.HasRevision {
		return -1
	}
	if !right.HasRevision {
		return 1
	}

	return compareInts(left.Revision, right.Revision)
}

func compareIdentifier(left string, right string) int {
	leftNumeric := isNumericIdentifier(left)
	rightNumeric := isNumericIdentifier(right)

	if leftNumeric && rightNumeric {
		leftNumber, _ := strconv.Atoi(left)
		rightNumber, _ := strconv.Atoi(right)
		return compareInts(leftNumber, rightNumber)
	}
	if leftNumeric {
		return -1
	}
	if rightNumeric {
		return 1
	}

	return compareInts(strings.Compare(left, right), 0)
}

func isNumericIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
