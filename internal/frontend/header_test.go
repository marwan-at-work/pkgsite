// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package frontend

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/net/html"
	"golang.org/x/pkgsite/internal"
	"golang.org/x/pkgsite/internal/middleware"
	"golang.org/x/pkgsite/internal/testing/htmlcheck"
	"golang.org/x/pkgsite/internal/testing/sample"
)

func samplePackage(mutators ...func(*Package)) *Package {
	p := &Package{
		Path:              sample.PackagePath,
		Synopsis:          sample.Synopsis,
		IsRedistributable: true,
		Licenses:          transformLicenseMetadata(sample.LicenseMetadata),
		Module: Module{
			DisplayVersion:    sample.VersionString,
			LinkVersion:       sample.VersionString,
			CommitTime:        "0 hours ago",
			ModulePath:        sample.ModulePath,
			IsRedistributable: true,
			Licenses:          transformLicenseMetadata(sample.LicenseMetadata),
		},
	}
	for _, mut := range mutators {
		mut(p)
	}
	p.URL = constructPackageURL(p.Path, p.ModulePath, p.LinkVersion)
	p.Module.URL = constructModuleURL(p.ModulePath, p.LinkVersion)
	p.LatestURL = constructPackageURL(p.Path, p.ModulePath, middleware.LatestVersionPlaceholder)
	p.Module.LatestURL = constructModuleURL(p.ModulePath, middleware.LatestVersionPlaceholder)
	return p
}

func TestElapsedTime(t *testing.T) {
	now := sample.NowTruncated()
	testCases := []struct {
		name        string
		date        time.Time
		elapsedTime string
	}{
		{
			name:        "one_hour_ago",
			date:        now.Add(time.Hour * -1),
			elapsedTime: "1 hour ago",
		},
		{
			name:        "hours_ago",
			date:        now.Add(time.Hour * -2),
			elapsedTime: "2 hours ago",
		},
		{
			name:        "today",
			date:        now.Add(time.Hour * -8),
			elapsedTime: "today",
		},
		{
			name:        "one_day_ago",
			date:        now.Add(time.Hour * 24 * -1),
			elapsedTime: "1 day ago",
		},
		{
			name:        "days_ago",
			date:        now.Add(time.Hour * 24 * -5),
			elapsedTime: "5 days ago",
		},
		{
			name:        "more_than_6_days_ago",
			date:        now.Add(time.Hour * 24 * -14),
			elapsedTime: now.Add(time.Hour * 24 * -14).Format("Jan _2, 2006"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			elapsedTime := elapsedTime(tc.date)

			if elapsedTime != tc.elapsedTime {
				t.Errorf("elapsedTime(%q) = %s, want %s", tc.date, elapsedTime, tc.elapsedTime)
			}
		})
	}
}

func TestCreatePackageHeader(t *testing.T) {
	vpkg := func(modulePath, suffix, name string) *internal.LegacyVersionedPackage {
		vp := &internal.LegacyVersionedPackage{
			LegacyModuleInfo: *sample.LegacyModuleInfo(modulePath, sample.VersionString),
			LegacyPackage:    *sample.LegacyPackage(modulePath, suffix),
		}
		if name != "" {
			vp.LegacyPackage.Name = name
		}
		return vp
	}

	for _, tc := range []struct {
		label   string
		pkg     *internal.LegacyVersionedPackage
		wantPkg *Package
	}{
		{
			label:   "simple package",
			pkg:     vpkg(sample.ModulePath, sample.Suffix, ""),
			wantPkg: samplePackage(),
		},
		{
			label:   "command package",
			pkg:     vpkg(sample.ModulePath, sample.Suffix, "main"),
			wantPkg: samplePackage(),
		},
		{
			label: "v2 command",
			pkg:   vpkg("pa.th/to/foo/v2", "bar", "main"),
			wantPkg: samplePackage(func(p *Package) {
				p.Path = "pa.th/to/foo/v2/bar"
				p.ModulePath = "pa.th/to/foo/v2"
			}),
		},
		{
			label: "explicit v1 command",
			pkg:   vpkg("pa.th/to/foo/v1", "", "main"),
			wantPkg: samplePackage(func(p *Package) {
				p.Path = "pa.th/to/foo/v1"
				p.ModulePath = "pa.th/to/foo/v1"
			}),
		},
	} {
		t.Run(tc.label, func(t *testing.T) {
			got, err := legacyCreatePackage(&tc.pkg.LegacyPackage, &tc.pkg.ModuleInfo, false, false)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.wantPkg, got); diff != "" {
				t.Errorf("legacyCreatePackage(%v) mismatch (-want +got):\n%s", tc.pkg, diff)
			}
		})
	}
}

func TestBreadcrumbPath(t *testing.T) {
	var (
		in    = htmlcheck.In
		notIn = htmlcheck.NotIn
		text  = htmlcheck.HasText
		attr  = htmlcheck.HasAttr
		href  = func(val string) htmlcheck.Checker { return htmlcheck.HasAttr("href", val) }
	)
	for _, test := range []struct {
		pkgPath, modPath, version string
		want                      htmlcheck.Checker
	}{
		{
			"example.com/blob/s3blob", "example.com", internal.LatestVersion,
			in("",
				in("a:nth-of-type(1)", href("/example.com"), text("example.com")),
				in("a:nth-of-type(2)", href("/example.com/blob"), text("blob")),
				in("span.DetailsHeader-breadcrumbCurrent", text("s3blob"))),
		},
		{
			"example.com", "example.com", internal.LatestVersion,
			in("",
				notIn("a"),
				in("span.DetailsHeader-breadcrumbCurrent", text("example.com"))),
		},

		{
			"g/x/tools/go/a", "g/x/tools", internal.LatestVersion,
			in("",
				in("a:nth-of-type(1)", href("/g/x/tools"), text("g/x/tools")),
				in("a:nth-of-type(2)", href("/g/x/tools/go"), text("go")),
				in("span.DetailsHeader-breadcrumbCurrent", text("a"))),
		},
		{
			"golang.org/x/tools", "golang.org/x/tools", internal.LatestVersion,
			in("",
				notIn("a"),
				in("span.DetailsHeader-breadcrumbCurrent", text("golang.org/x/tools"))),
		},
		{
			// Special case: stdlib.
			"encoding/json", "std", internal.LatestVersion,
			in("",
				in("a", href("/encoding"), text("encoding")),
				in("span.DetailsHeader-breadcrumbCurrent", text("json"))),
		},
		{
			"example.com/blob/s3blob", "example.com", "v1",
			in("",
				in("a:nth-of-type(1)", href("/example.com@v1"), text("example.com")),
				in("a:nth-of-type(2)", href("/example.com/blob@v1"), text("blob")),
				in("span.DetailsHeader-breadcrumbCurrent", text("s3blob"))),
		},
	} {
		t.Run(fmt.Sprintf("%s-%s-%s", test.pkgPath, test.modPath, test.version), func(t *testing.T) {
			want := in("div.DetailsHeader-breadcrumb",
				test.want,
				in("button.js-detailsHeaderCopyPath",
					attr("aria-label", "Copy path to clipboard"),
					in("svg > title", text("Copy path to clipboard"))),
				in("input.DetailsHeader-pathInput",
					attr("role", "presentation"),
					attr("tabindex", "-1"),
					attr("value", test.pkgPath)))

			got := breadcrumbPath(test.pkgPath, test.modPath, test.version)
			doc, err := html.Parse(strings.NewReader(string(got)))
			if err != nil {
				t.Fatal(err)
			}
			if err := want(doc); err != nil {
				t.Error(err)
			}
		})
	}
}
