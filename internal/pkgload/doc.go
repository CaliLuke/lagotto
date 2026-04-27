// Package pkgload wraps golang.org/x/tools/go/packages.Load with
// lagotto's --tags and --exclude conventions. Detectors receive the
// already-filtered slice of *packages.Package and do not interact
// with the loader directly.
package pkgload
