// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package pagecheck implements HTML checkers for discovery site pages.
// It uses the general-purpose checkers in internal/testing/htmlcheck to define
// site-specific checkers.
package pagecheck

import (
	"fmt"
	"path"
	"regexp"

	"golang.org/x/discovery/internal/stdlib"
	"golang.org/x/discovery/internal/testing/htmlcheck"
)

// Page describes a discovery site web page for a package, module or directory.
type Page struct {
	ModulePath       string
	Suffix           string // package or directory path after module path; empty for a module
	Version          string
	Title            string
	LicenseType      string
	IsLatest         bool   // is this the latest version of this module?
	LatestLink       string // href of "Go to latest" link
	PackageURLFormat string // the relative package URL, with one %s for "@version"; also used for dirs
	ModuleURL        string // the relative module URL
}

var (
	in        = htmlcheck.In
	inAt      = htmlcheck.InAt
	text      = htmlcheck.HasText
	exactText = htmlcheck.HasExactText
	attr      = htmlcheck.HasAttr
	href      = htmlcheck.HasHref
)

// PackageHeader checks a details page header for a package.
func PackageHeader(p *Page, versionedURL bool) htmlcheck.Checker {
	return in("",
		in("span.DetailsHeader-breadcrumbCurrent", exactText(path.Base(p.Suffix))),
		in("h1.DetailsHeader-title", exactText(p.Title)),
		in("div.DetailsHeader-version", exactText(p.Version)),
		versionBadge(p),
		licenseInfo(p, packageURLPath(p, versionedURL), versionedURL),
		moduleInHeader(p, versionedURL))
}

// ModuleHeader checks a details page header for a module.
func ModuleHeader(p *Page, versionedURL bool) htmlcheck.Checker {
	return in("",
		in("h1.DetailsHeader-title", exactText(p.Title)),
		in("div.DetailsHeader-version", exactText(p.Version)),
		licenseInfo(p, moduleURLPath(p, versionedURL), versionedURL))
}

// DirectoryHeader checks a details page header for a directory.
func DirectoryHeader(p *Page, versionedURL bool) htmlcheck.Checker {
	return in("",
		in("span.DetailsHeader-breadcrumbCurrent", exactText(path.Base(p.Suffix))),
		in("h1.DetailsHeader-title", exactText(p.Title)),
		in("div.DetailsHeader-version", exactText(p.Version)),
		// directory pages don't show a header badge
		in("div.DetailsHeader-badge", in(".DetailsHeader-unknown")),
		licenseInfo(p, packageURLPath(p, versionedURL), versionedURL),
		// directory module links are always versioned (see b/144217401)
		moduleInHeader(p, true))
}

// SubdirectoriesDetails checks the detail section of a subdirectories tab.
func SubdirectoriesDetails() htmlcheck.Checker {
	return in("",
		inAt("th", 0, text("^Path$")),
		inAt("th", 1, text("^Synopsis$")))
}

// LicenseDetails checks the details section of a license tab.
func LicenseDetails(ltype, bodySubstring, source string) htmlcheck.Checker {
	return in("",
		in(".License",
			text(regexp.QuoteMeta(ltype)),
			text("This is not legal advice"),
			in("a",
				href("/license-policy"),
				exactText("Read disclaimer.")),
			in(".License-contents",
				text(regexp.QuoteMeta(bodySubstring)))),
		in(".License-source",
			exactText("Source: "+source)))
}

// versionBadge checks the latest-version badge on a header.
func versionBadge(p *Page) htmlcheck.Checker {
	class := "DetailsHeader-"
	if p.IsLatest {
		class += "latest"
	} else {
		class += "goToLatest"
	}
	return in("div.DetailsHeader-badge",
		attr("class", `\b`+regexp.QuoteMeta(class)+`\b`), // the badge has this class too
		in("a", href(p.LatestLink), exactText("Go to latest")))
}

// licenseInfo checks the license part of the info label in the header.
func licenseInfo(p *Page, urlPath string, versionedURL bool) htmlcheck.Checker {
	if p.LicenseType == "" {
		return inAt("div.DetailsHeader-infoLabel > span", 3, text("None detected"))
	}
	return inAt("div.DetailsHeader-infoLabel > span", 3,
		in("a",
			href(fmt.Sprintf("%s?tab=licenses#LICENSE", urlPath)),
			exactText(p.LicenseType)))
}

// moduleInHeader checks the module part of the info label in the header.
func moduleInHeader(p *Page, versionedURL bool) htmlcheck.Checker {
	modURL := moduleURLPath(p, versionedURL)
	if p.ModulePath == stdlib.ModulePath {
		return in("div.DetailsHeader-infoLabel", inAt("a", 1, href(modURL), exactText("Standard library")))
	}
	return inAt("div.DetailsHeader-infoLabel > span", 6, in("a", href(modURL), exactText(p.ModulePath)))
}

func packageURLPath(p *Page, versioned bool) string {
	v := ""
	if versioned {
		v = "@" + p.Version
	}
	return fmt.Sprintf(p.PackageURLFormat, v)
}

func moduleURLPath(p *Page, versioned bool) string {
	if versioned {
		return p.ModuleURL + "@" + p.Version
	}
	return p.ModuleURL
}
