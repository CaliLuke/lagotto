package detect

import "testing"

func foreignHolderFixture() map[string]string {
	return map[string]string{
		"store/users/users.go":     "package users\n\ntype Service struct{}\n",
		"store/billing/billing.go": "package billing\n\ntype Service struct{}\n",
		"store/reports/reports.go": "package reports\n\ntype Service struct{}\n",
		"store/store.go": `package store

import (
	"example.com/test/store/billing"
	"example.com/test/store/reports"
	"example.com/test/store/users"
)

type Store struct {
	Users *users.Service
	Billing *billing.Service
	Reports *reports.Service
}
`,
	}
}

func TestG1E_HolderInForeignSignature_Fires(t *testing.T) {
	files := foreignHolderFixture()
	files["app/app.go"] = `package app

import "example.com/test/store"

func Handle(s *store.Store) {}
`
	findings := ScanReceivers(fakeModule(t, files))
	if !containsID(findings, "G1E") {
		t.Fatalf("expected G1E, got %v", findingIDs(findings))
	}
}

func TestG1E_HolderInForeignStructField_Fires(t *testing.T) {
	files := foreignHolderFixture()
	files["app/app.go"] = `package app

import "example.com/test/store"

type Deps struct { Store *store.Store }
`
	findings := ScanReceivers(fakeModule(t, files))
	if !containsID(findings, "G1E") {
		t.Fatalf("expected G1E, got %v", findingIDs(findings))
	}
}

func TestG1E_AccessorPattern_Fires(t *testing.T) {
	files := foreignHolderFixture()
	files["store/store.go"] = `package store

import (
	"example.com/test/store/billing"
	"example.com/test/store/reports"
	"example.com/test/store/users"
)

type Store struct{}
func Users(*Store) *users.Service { return nil }
func Billing(*Store) *billing.Service { return nil }
func Reports(*Store) *reports.Service { return nil }
`
	files["app/app.go"] = "package app\n\nimport \"example.com/test/store\"\n\nfunc Handle(s *store.Store) {}\n"
	findings := ScanReceivers(fakeModule(t, files))
	if !containsID(findings, "G1E") {
		t.Fatalf("expected G1E, got %v", findingIDs(findings))
	}
}

func TestG1E_ProducerAndConstructor_NoFire(t *testing.T) {
	files := foreignHolderFixture()
	files["store/local.go"] = "package store\n\nfunc Use(s *Store) {}\n"
	files["builder/builder.go"] = `package builder

import "example.com/test/store"

func NewStore() *store.Store { return nil }
`
	findings := ScanReceivers(fakeModule(t, files))
	if containsID(findings, "G1E") {
		t.Fatalf("did not expect G1E for producer use or a constructor, got %+v", findings)
	}
}

func TestG1E_TestFixtureAndRegularType_NoFire(t *testing.T) {
	files := foreignHolderFixture()
	files["testutil/fixture.go"] = "package testutil\n\nimport \"example.com/test/store\"\n\nfunc Fixture(s *store.Store) {}\n"
	files["plain/plain.go"] = "package plain\n\ntype Conn struct{}\n"
	files["app/app.go"] = "package app\n\nimport \"example.com/test/plain\"\n\nfunc Handle(c *plain.Conn) {}\n"
	findings := ScanReceivers(fakeModule(t, files))
	if containsID(findings, "G1E") {
		t.Fatalf("did not expect G1E for test fixtures or regular types, got %+v", findings)
	}
}
