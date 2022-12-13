package tests

import (
	"os"
	"ronald-destroyer/ronnyd"
	"testing"
)

func TestMain(m *testing.M) {
	ronnyd.LoadConfig()
	LoadDevFixtures(m)
	os.Exit(m.Run())
}

func TestDatabaseIsNotEmpty(t *testing.T) {
	db := ronnyd.ConnectToDB()
	var authors []*ronnyd.Author
	db.Table("authors").Find(&authors)
	if len(authors) == 0 {
		t.Fail()
	}
}
