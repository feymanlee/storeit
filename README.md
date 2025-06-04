# storeit

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/feymanlee/storeit?style=flat-square)
[![Go Report Card](https://goreportcard.com/badge/github.com/feymanlee/storeit)](https://goreportcard.com/report/github.com/feymanlee/cacheit)
[![Unit-Tests](https://github.com/feymanlee/storeit/workflows/Unit-Tests/badge.svg)](https://github.com/feymanlee/storeit/actions)
[![codecov](https://codecov.io/gh/feymanlee/storeit/graph/badge.svg?token=S8M4CFG0NB)](https://codecov.io/gh/feymanlee/storeit)
[![Go Reference](https://pkg.go.dev/badge/github.com/feymanlee/storeit.svg)](https://pkg.go.dev/github.com/feymanlee/cacheit)
[![License](https://img.shields.io/github/license/feymanlee/storeit)](./LICENSE)

## Tag List
| Tag           | Value Type     | SQL Statment                        | DESC                   |
|---------------|----------------|-------------------------------------|------------------------|
| field:eq      | any            | feild = value                       |                        |
| field:neq     | any            | feild <> value                      |                        |
| field:gt      | any            | feild > value                       |                        |
| field:gte     | any            | feild >= value                      |                        |
| field:lt      | any            | feild < value                       |                        |
| field:lte     | any            | feild <= value                      |                        |
| field:like    | string         | feild LIKE "%value%"                |                        |
| field:llike   | string         | feild LIKE "%value"                 |                        |
| field:rlike   | string         | feild LIKE "value%"                 |                        |
| field:in      | []any          | feild IN (value)                    |                        |
| field:notin   | []any          | feild IN (value)                    |                        |
| field:isnull  | any            | feild IS NULL                       |                        |
| field:notnull | []any          | feild IS NOT NULL                   |                        |
| field:between | []any (len==2) | feild BETWEEN value[0] AND value[1] |                        |
| -:sort        | string         | ORDER BY a DESC, b, c DESC          | value is a-,b+,c-      |
| -:page        | int            | OFFSET (value-1)*per_page           | Default per_page is 50 |
| -:per_page    | int            | LIMIT value                         | Default  50            |
| -:limit       | int            | LIMIT value                         |                        |
| -:offset      | int            | OFFSET value                        |                        |

## 在 gin 里面使用
```go
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/feymanlee/storeit"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type User struct {
	ID        int64        `gorm:"column:id;primarykey" json:"id"`
	Username  string       `gorm:"column:username" json:"username"`
	Email     string       `gorm:"column:email" json:"email"`
	Mobile    string       `gorm:"column:mobile" json:"mobile"`
	Status    string       `gorm:"column:status" json:"status"`
	Weight    int          `gorm:"column:weight" json:"weight"`
	Source    string       `gorm:"column:source" json:"source"`
	CreatedAt time.Time    `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time    `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt sql.NullTime `gorm:"column:email;index"`
}

var db *gorm.DB

func init() {
	var err error
	db, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		panic(err)
	}
}

func main() {
	router := gin.Default()
	v1 := router.Group("/api/v1")
	{
		v1.GET("/users", SearchUser)
		v1.POST("/users", CreateUser)
		v1.GET("/user/:id", FindUser)
		v1.PUT("/users/:id", UpdateUser)
		v1.DELETE("/users/:id", DeleteUser)
	}
	router.Run(":8180")
}
func SearchUser(c *gin.Context) {
	var req struct {
		ID      int    `form:"id" criteria:"id,eq"`
		Keyword string `form:"keyword" criteria:"phone,email:like"`
		Name    string `form:"name" criteria:"name:llike"`
		Phone   int    `form:"phone" criteria:"phone:eq"`
		Status  string `form:"status" criteria:"status:eq"`
		Weight  int    `form:"weight" criteria:"weight:eq"`
		Source  string `form:"source"`
		Page    int    `form:"page" criteria:"-:page"`
		PerPage int    `form:"per_page" criteria:"-:per_page"`
		Sorts   string `form:"sorts" criteria:"-:sort"`
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		log.Println(err)
	}
	criteria, _ := storeit.ExtractCriteria(req)
	// source 是多个值，使用英文逗号分隔
	criteria.WhereIn("source", strings.Split(req.Source, ","))
	// 实现自动分页
	ret, _ := storeit.New[User](db).Paginate(c, criteria)

	c.JSON(http.StatusOK, ret)
}

func FindUser(c *gin.Context) {
	user, _ := storeit.New[User](db).FindByID(c, c.Params.ByName("id"))

	c.JSON(200, user)
}

func CreateUser(c *gin.Context) {
	var user User
	_ = c.ShouldBindJSON(&user)
	storeit.New[User](db).Insert(c, &user)
	c.JSON(200, user)
}

func UpdateUser(c *gin.Context) {
	var user User
	_ = c.ShouldBindJSON(&user)
	id, _ := strconv.Atoi(c.Params.ByName("id"))
	user.ID = int64(id)
	storeit.New[User](db).Save(c, &user)
	c.JSON(200, user)
}

func DeleteUser(c *gin.Context) {
	tx := storeit.New[User](db).DeleteById(c, c.Params.ByName("id"))
	if tx.Error != nil {
		c.AbortWithStatus(http.StatusBadRequest)
	}
	c.JSON(200, gin.H{
		"id": c.Params.ByName("id"),
	})
}
```

