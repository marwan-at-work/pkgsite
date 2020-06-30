// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package frontend

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/pkgsite/internal"
	"golang.org/x/pkgsite/internal/derrors"
	"golang.org/x/pkgsite/internal/postgres"
	"golang.org/x/pkgsite/internal/stdlib"
	"golang.org/x/pkgsite/internal/testing/sample"
)

func TestFetchDirectoryDetails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	defer postgres.ResetTestDB(testDB, t)
	postgres.InsertSampleDirectoryTree(ctx, t, testDB)

	checkDirectory := func(got *Directory, dirPath, modulePath, version string, suffixes []string) {
		t.Helper()

		mi := sample.ModuleInfo(modulePath, version)
		var wantPkgs []*Package
		for _, suffix := range suffixes {
			sp := sample.LegacyPackage(modulePath, suffix)
			pkg, err := legacyCreatePackage(sp, mi, false, false)
			if err != nil {
				t.Fatal(err)
			}
			pkg.PathAfterDirectory = strings.TrimPrefix(strings.TrimPrefix(sp.Path, dirPath), "/")
			if pkg.PathAfterDirectory == "" {
				pkg.PathAfterDirectory = fmt.Sprintf("%s (root)", effectiveName(sp))
			}
			wantPkgs = append(wantPkgs, pkg)
		}

		mod := createModule(mi, sample.LicenseMetadata, false)
		want := &Directory{
			Module:   *mod,
			Path:     dirPath,
			Packages: wantPkgs,
			URL:      constructDirectoryURL(dirPath, mi.ModulePath, linkVersion(mi.Version, mi.ModulePath)),
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("fetchDirectoryDetails(ctx, %q, %q, %q) mismatch (-want +got):\n%s", dirPath, modulePath, version, diff)
		}
	}

	for _, tc := range []struct {
		name, dirPath, modulePath, version, wantModulePath, wantVersion string
		wantPkgSuffixes                                                 []string
		includeDirPath, wantInvalidArgumentErr                          bool
	}{
		{
			name:            "dirPath is modulePath, includeDirPath = true, want longest module path",
			includeDirPath:  true,
			dirPath:         "github.com/hashicorp/vault/api",
			modulePath:      "github.com/hashicorp/vault/api",
			version:         internal.LatestVersion,
			wantModulePath:  "github.com/hashicorp/vault/api",
			wantVersion:     "v1.1.2",
			wantPkgSuffixes: []string{""},
		},
		{
			name:            "only dirPath provided, includeDirPath = false, want longest module path",
			dirPath:         "github.com/hashicorp/vault/api",
			modulePath:      internal.UnknownModulePath,
			version:         internal.LatestVersion,
			wantModulePath:  "github.com/hashicorp/vault/api",
			wantVersion:     "v1.1.2",
			wantPkgSuffixes: nil,
		},
		{
			name:            "dirPath@version, includeDirPath = false, want longest module path",
			dirPath:         "github.com/hashicorp/vault/api",
			modulePath:      internal.UnknownModulePath,
			version:         "v1.1.2",
			wantModulePath:  "github.com/hashicorp/vault/api",
			wantVersion:     "v1.1.2",
			wantPkgSuffixes: nil,
		},
		{
			name:            "dirPath@version,  includeDirPath = false, version only exists for shorter module path",
			dirPath:         "github.com/hashicorp/vault/api",
			modulePath:      internal.UnknownModulePath,
			version:         "v1.0.3",
			wantModulePath:  "github.com/hashicorp/vault",
			wantVersion:     "v1.0.3",
			wantPkgSuffixes: nil,
		},
		{
			name:           "valid directory for modulePath@version/suffix, includeDirPath = false",
			dirPath:        "github.com/hashicorp/vault/builtin",
			modulePath:     "github.com/hashicorp/vault",
			version:        "v1.0.3",
			wantModulePath: "github.com/hashicorp/vault",
			wantVersion:    "v1.0.3",
			wantPkgSuffixes: []string{
				"builtin/audit/file",
				"builtin/audit/socket",
			},
		},
		{
			name:           "standard library",
			dirPath:        stdlib.ModulePath,
			modulePath:     stdlib.ModulePath,
			version:        "v1.13.4",
			wantModulePath: stdlib.ModulePath,
			wantVersion:    "v1.13.4",
			wantPkgSuffixes: []string{
				"archive/tar",
				"archive/zip",
				"cmd/go",
				"cmd/internal/obj",
				"cmd/internal/obj/arm",
				"cmd/internal/obj/arm64",
			},
		},
		{
			name:           "cmd",
			dirPath:        "cmd",
			modulePath:     stdlib.ModulePath,
			version:        "v1.13.4",
			wantModulePath: stdlib.ModulePath,
			wantVersion:    "v1.13.4",
			wantPkgSuffixes: []string{
				"cmd/go",
				"cmd/internal/obj",
				"cmd/internal/obj/arm",
				"cmd/internal/obj/arm64",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mi := sample.ModuleInfoReleaseType(tc.modulePath, tc.version)
			got, err := fetchDirectoryDetails(ctx, testDB,
				tc.dirPath, mi, sample.LicenseMetadata, tc.includeDirPath, false)
			if err != nil {
				t.Fatal(err)
			}
			checkDirectory(got, tc.dirPath, tc.wantModulePath, tc.wantVersion, tc.wantPkgSuffixes)
		})
	}
}

func TestFetchDirectoryDetailsInvalidArguments(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	defer postgres.ResetTestDB(testDB, t)
	postgres.InsertSampleDirectoryTree(ctx, t, testDB)

	for _, tc := range []struct {
		name, dirPath, modulePath, version, wantModulePath, wantVersion string
		includeDirPath                                                  bool
		wantPkgPaths                                                    []string
	}{
		{
			name:       "dirPath is empty",
			dirPath:    "github.com/hashicorp/vault/api",
			modulePath: "",
			version:    internal.LatestVersion,
		},
		{
			name:       "modulePath is empty",
			dirPath:    "github.com/hashicorp/vault/api",
			modulePath: "",
			version:    internal.LatestVersion,
		},
		{
			name:       "version is empty",
			dirPath:    "github.com/hashicorp/vault/api",
			modulePath: internal.UnknownModulePath,
			version:    "",
		},
		{
			name:           "dirPath is not modulePath, includeDirPath = true",
			dirPath:        "github.com/hashicorp/vault/api",
			modulePath:     "github.com/hashicorp/vault",
			version:        internal.LatestVersion,
			includeDirPath: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mi := sample.ModuleInfoReleaseType(tc.modulePath, tc.version)
			got, err := fetchDirectoryDetails(ctx, testDB,
				tc.dirPath, mi, sample.LicenseMetadata, tc.includeDirPath, false)
			if !errors.Is(err, derrors.InvalidArgument) {
				t.Fatalf("expected err; got = \n%+v, %v", got, err)
			}
		})
	}
}
