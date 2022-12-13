package tests

import (
	"fmt"
	"ronald-destroyer/ronnyd"
)

func LoadDevFixtures() {
	ronnyd.LoadConfig()
	db := ronnyd.ConnectToDB()
	var authors []*ronnyd.Author
	db.Table("authors").Find(&authors)
	fmt.Println(authors)
	
	//file, err := ioutil.ReadFile("testuitl/fixtures/authors.json")
}