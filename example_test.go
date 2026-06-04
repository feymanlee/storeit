package storeit_test

import (
	"context"
	"fmt"

	"github.com/feymanlee/storeit"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type exampleUser struct {
	ID     int64 `gorm:"primaryKey"`
	Name   string
	Status string
}

func ExampleNew() {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	if err := db.AutoMigrate(&exampleUser{}); err != nil {
		panic(err)
	}

	ctx := context.Background()
	users := storeit.New[exampleUser](db)

	alice := exampleUser{Name: "Alice", Status: "active"}
	if err := users.Create(ctx, &alice).Error; err != nil {
		panic(err)
	}

	page, err := users.Paginate(ctx, storeit.NewCriteria().Where("status = ?", "active"))
	if err != nil {
		panic(err)
	}

	fmt.Println(page.Total, page.Items[0].Name)
	// Output: 1 Alice
}

func ExampleExtractCriteria() {
	type searchUsers struct {
		Status string `criteria:"status:eq"`
		Sort   string `criteria:"-:sort"`
		Page   int    `criteria:"-:page"`
	}

	criteria, err := storeit.ExtractCriteria(searchUsers{
		Status: "active",
		Sort:   "id-",
		Page:   2,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(criteria.GetPage())
	// Output: 2
}
