// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRefusesNamesConstNameCannotHandle(t *testing.T) {
	for _, name := range []string{"a_", "a__b", "_a", "A", "a-b", "1a", ""} {
		v := vocabulary{Subject: []label{{Name: name, Description: "x"}}}
		if err := validate(v); err == nil {
			t.Errorf("%q was accepted", name)
		}
	}
}

func TestValidateAcceptsSnakeCase(t *testing.T) {
	for _, name := range []string{"a", "project", "resource_name", "node_role2", "build_version"} {
		v := vocabulary{Subject: []label{{Name: name, Description: "x"}}}
		if err := validate(v); err != nil {
			t.Errorf("%q was refused: %v", name, err)
		}
		if constName(name) == "" {
			t.Errorf("constName(%q) is empty", name)
		}
	}
}

func TestValidateRefusesALabelInTwoGroups(t *testing.T) {
	v := vocabulary{
		Subject:  []label{{Name: "cluster", Description: "x"}},
		Reserved: []label{{Name: "cluster", Description: "x"}},
	}
	if err := validate(v); err == nil {
		t.Error("a label in both subject and reserved was accepted")
	}
}

func TestValidateRefusesLabelsThatGenerateClashingIdentifiers(t *testing.T) {
	for name, v := range map[string]vocabulary{
		"same constant":    {Subject: []label{{Name: "x_1", Description: "x"}, {Name: "x1", Description: "x"}}},
		"package function": {Subject: []label{{Name: "is_reserved", Description: "x"}}},
		"across groups":    {Subject: []label{{Name: "x_1", Description: "x"}}, Build: []label{{Name: "x1", Description: "x"}}},
	} {
		if err := validate(v); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestValidateAllowsReservedNamesThatMatchPackageIdentifiers(t *testing.T) {
	v := vocabulary{Reserved: []label{{Name: "reserved", Description: "x"}}}
	if err := validate(v); err != nil {
		t.Errorf("a reserved label generates no constant, so it cannot clash: %v", err)
	}
}

func TestGenerateTSKeepsDescriptionsInsideTheirComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "labels.yaml")
	yaml := `subject:
  - name: project
    description: "Ends a comment */ early, and quotes \"this\"."
build:
  - name: build_version
    description: Version.
reserved:
  - name: cluster
    description: "Quotes \"this\"."
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := generateTS(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(src)
	for _, want := range []string{
		`/** The "project" label. Ends a comment *\/ early, and quotes "this". */`,
		`export const Project = "project";`,
		`export const BuildVersion = "build_version";`,
		`  "cluster": "Quotes \"this\".",`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated TypeScript lacks %s:\n%s", want, got)
		}
	}
	if strings.Count(got, "*/") != strings.Count(got, "/**") {
		t.Errorf("a description closed its comment early:\n%s", got)
	}
}
