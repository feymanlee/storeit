package storeit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type testCriteriaStruct struct {
	Name     string `criteria:"name:eq"`
	Age      int    `criteria:"age:gt"`
	Email    string `criteria:"email:like"`
	Status   string `criteria:"status1,status2:eq"`
	Page     int    `criteria:"page:page"`
	PerPage  int    `criteria:"per_page:per_page"`
	SortBy   string `criteria:"sort:sort"`
	Offset   int    `criteria:"offset:offset"`
	Limit    int    `criteria:"limit:limit"`
	Keywords string `criteria:"title,content:like"`
}

func dryRunCriteriaSQL(t *testing.T, criteria *Criteria) string {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	assert.NoError(t, err)

	tx := db.Session(&gorm.Session{DryRun: true}).Model(&TestModel{})
	if criteria != nil {
		if criteria.GetOffset() > 0 {
			tx = tx.Offset(criteria.GetOffset())
		}
		if criteria.limit > 0 || criteria.GetOffset() > 0 {
			tx = tx.Limit(criteria.limit)
		}
		if criteria.group != "" {
			tx = tx.Group(criteria.group)
		}
		for _, item := range criteria.orders {
			tx = tx.Order(item)
		}
		for _, closure := range criteria.scopeClosures {
			tx = closure(tx)
		}
	}

	tx = tx.Find(&[]TestModel{})
	return tx.Statement.SQL.String()
}

func TestExtractCriteria(t *testing.T) {
	// nil
	_, err := ExtractCriteria(nil)
	assert.Error(t, err)

	// 非结构体
	_, err = ExtractCriteria("not struct")
	assert.Error(t, err)

	// 指针为nil
	var ptr *testCriteriaStruct
	_, err = ExtractCriteria(ptr)
	assert.Error(t, err)

	// 正常结构体
	s := testCriteriaStruct{
		Name:     "n",
		Age:      18,
		Email:    "e",
		Status:   "active",
		Page:     2,
		PerPage:  10,
		SortBy:   "name-,age+",
		Offset:   5,
		Limit:    20,
		Keywords: "kw",
	}
	c, err := ExtractCriteria(s)
	assert.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, 2, c.GetPage())
	assert.Equal(t, 20, c.GetPerPage())
	assert.Equal(t, 5, c.GetOffset())
	assert.Equal(t, 20, c.GetLimit())
	assert.NotEmpty(t, c.orders)
	assert.NotEmpty(t, c.scopeClosures)

	// 错误tag
	type badTag struct {
		Foo string `criteria:"badtag"`
	}
	_, err = ExtractCriteria(badTag{Foo: "bar"})
	assert.Error(t, err)
}

func TestCriteria_WhereAndOr(t *testing.T) {
	c := NewCriteria()
	c.Where("name = ?", "foo")
	c.OrWhere("age = ?", 18)

	sql := dryRunCriteriaSQL(t, c)
	assert.Contains(t, sql, "name = ?")
	assert.Contains(t, sql, "OR age = ?")
}

func TestCriteria_WhereGtGteLtLte(t *testing.T) {
	c := NewCriteria()
	c.WhereGt("age", 10)
	c.WhereGte("age", 11)
	c.WhereLt("age", 20)
	c.WhereLte("age", 21)

	sql := dryRunCriteriaSQL(t, c)
	assert.Contains(t, sql, "age > ?")
	assert.Contains(t, sql, "age >= ?")
	assert.Contains(t, sql, "age < ?")
	assert.Contains(t, sql, "age <= ?")
}

func TestCriteria_WhereNotAndNull(t *testing.T) {
	c := NewCriteria()
	c.WhereNot("name", "foo")
	c.WhereIsNull("email")
	c.WhereNotNull("email")

	sql := dryRunCriteriaSQL(t, c)
	assert.Contains(t, sql, "`name` <> ?")
	assert.Contains(t, sql, "email IS NULL")
	assert.Contains(t, sql, "email IS NOT NULL")
}

func TestCriteria_WhereInNotIn(t *testing.T) {
	c := NewCriteria()
	c.WhereIn("status", []string{"a", "b"})
	c.WhereNotIn("status", []string{"c"})

	sql := dryRunCriteriaSQL(t, c)
	assert.Contains(t, sql, "status IN")
	assert.Contains(t, sql, "status NOT IN")
}

func TestCriteria_WhereStartEndContainsBetween(t *testing.T) {
	c := NewCriteria()
	c.WhereStartWith("name", "A")
	c.WhereEndWith("name", "Z")
	c.WhereContains("desc", "foo")
	c.WhereBetween("age", 1, 10)

	sql := dryRunCriteriaSQL(t, c)
	assert.Contains(t, sql, "name LIKE ?")
	assert.Contains(t, sql, "`desc` LIKE ?")
	assert.Contains(t, sql, "age BETWEEN ? AND ?")
}

func TestCriteria_Order(t *testing.T) {
	c := NewCriteria()
	c.Order("name", true)
	c.OrderAsc("age")
	c.OrderDesc("score")
	assert.Equal(t, []string{"name DESC", "age", "score DESC"}, c.orders)
}

func TestCriteria_LimitOffsetPagePerPage(t *testing.T) {
	c := NewCriteria()
	c.Limit(10)
	c.Offset(5)
	c.Page(2)
	assert.Equal(t, 10, c.limit)
	c.PerPage(20)
	assert.Equal(t, 20, c.limit)
	assert.Equal(t, 5, c.offset)
	assert.Equal(t, 2, c.page)
}

func TestCriteria_GroupHavingJoinsPreload(t *testing.T) {
	c := NewCriteria()
	c.Group("status")
	c.Having("COUNT(*) > ?", 1)
	c.Joins("LEFT JOIN t ON t.id = a.id")

	sql := dryRunCriteriaSQL(t, c)
	assert.Contains(t, sql, "LEFT JOIN t ON t.id = a.id")
	assert.Contains(t, sql, "GROUP BY `status`")
	assert.Contains(t, sql, "HAVING COUNT(*) > ?")
}

func TestCriteria_GetPagePerPageOffsetLimit(t *testing.T) {
	c := NewCriteria().Page(3).PerPage(15)
	assert.Equal(t, 3, c.GetPage())
	assert.Equal(t, 15, c.GetPerPage())
	assert.Equal(t, 30, c.GetOffset())
	assert.Equal(t, 15, c.GetLimit())

	c = NewCriteria().Offset(7)
	assert.Equal(t, 7, c.GetOffset())
}

func TestCriteria_unsetOrderAndLimit(t *testing.T) {
	c := NewCriteria().Order("name", false).Limit(10).Offset(5)
	c.unsetOrder()
	assert.Empty(t, c.orders)
	c.unsetLimit()
	assert.Equal(t, 0, c.limit)
	assert.Equal(t, 0, c.offset)
}

func TestCriteria_buildConditionSpec(t *testing.T) {
	c := NewCriteria()
	// eq
	cond, err := c.buildConditionSpec("eq", "name", "foo")
	assert.NoError(t, err)
	assert.Equal(t, "name = ?", cond.query)
	assert.Equal(t, []any{"foo"}, cond.args)

	// gt
	cond, err = c.buildConditionSpec("gt", "age", 18)
	assert.NoError(t, err)
	assert.Equal(t, "age > ?", cond.query)

	// like
	cond, err = c.buildConditionSpec("like", "email", "bar")
	assert.NoError(t, err)
	assert.Equal(t, "email like ?", cond.query)
	assert.Equal(t, []any{"%bar%"}, cond.args)

	// llike
	cond, err = c.buildConditionSpec("llike", "email", "bar")
	assert.NoError(t, err)
	assert.Equal(t, []any{"%bar"}, cond.args)

	// rlike
	cond, err = c.buildConditionSpec("rlike", "email", "bar")
	assert.NoError(t, err)
	assert.Equal(t, []any{"bar%"}, cond.args)

	// unknown
	cond, err = c.buildConditionSpec("unknown", "foo", "bar")
	assert.NoError(t, err)
	assert.Empty(t, cond.query)
}

func TestBuildLikeCondition(t *testing.T) {
	cond := buildLikeCondition("name", "foo", criteriaLike)
	assert.Equal(t, "name like ?", cond.query)
	assert.Equal(t, []any{"%foo%"}, cond.args)

	cond = buildLikeCondition("name", "foo", criteriaLLike)
	assert.Equal(t, []any{"%foo"}, cond.args)

	cond = buildLikeCondition("name", "foo", criteriaRLike)
	assert.Equal(t, []any{"foo%"}, cond.args)
}

func TestCriteria_GroupOr(t *testing.T) {
	c := NewCriteria()
	group := groupConditionSpec{
		{query: "name = ?", args: []any{"foo"}},
		{query: "age > ?", args: []any{18}},
	}
	c.GroupOr(group)

	sql := dryRunCriteriaSQL(t, c)
	assert.Contains(t, sql, "name = ? OR age > ?")

	// 空组
	c2 := NewCriteria()
	c2.GroupOr(groupConditionSpec{})
	assert.NotContains(t, dryRunCriteriaSQL(t, c2), "name = ?")
}

func TestCriteria_ZeroValueFieldSkip(t *testing.T) {
	type S struct {
		Name string `criteria:"name:eq"`
		Age  int    `criteria:"age:gt"`
	}
	s := S{}
	c, err := ExtractCriteria(s)
	assert.NoError(t, err)
	assert.NotNil(t, c)
	assert.NotContains(t, dryRunCriteriaSQL(t, c), "name = ?")
	assert.NotContains(t, dryRunCriteriaSQL(t, c), "age > ?")
}

func TestCriteria_TagError(t *testing.T) {
	type S struct {
		Foo string `criteria:"badtag"`
	}
	_, err := ExtractCriteria(S{Foo: "bar"})
	assert.Error(t, err)
}

func TestCriteria_PageLessThanOne(t *testing.T) {
	c := NewCriteria().Page(-1)
	assert.Equal(t, 1, c.page)
}

func TestCriteria_PerPageAffectsLimit(t *testing.T) {
	c := NewCriteria().PerPage(99)
	assert.Equal(t, 99, c.limit)
}

func TestCriteria_WhereBetween(t *testing.T) {
	c := NewCriteria()
	c.WhereBetween("age", 1, 10)
	assert.Contains(t, dryRunCriteriaSQL(t, c), "age BETWEEN ? AND ?")
}

func TestCriteria_OrderReservedWord(t *testing.T) {
	c := NewCriteria()
	c.Order("order", false)
	assert.Contains(t, c.orders[0], "`order`")
}

func TestCriteria_WhereNeq(t *testing.T) {
	c := NewCriteria()
	c.WhereNeq("status", "deleted")
	assert.Len(t, c.scopeClosures, 1)
}

func TestCriteria_OrderDescAsc(t *testing.T) {
	c := NewCriteria()
	c.OrderDesc("created_at")
	c.OrderAsc("name")
	assert.Equal(t, []string{"created_at DESC", "name"}, c.orders)
}

func TestCriteria_ScopeClosure(t *testing.T) {
	c := NewCriteria()
	called := false
	c.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
		called = true
		return tx.Where("test = ?", 1)
	})
	assert.Len(t, c.scopeClosures, 1)
	assert.False(t, called) // Scope 应该延迟执行
}

func TestCriteria_AddPreload(t *testing.T) {
	c := NewCriteria()
	c.AddPreload("Orders")
	c.AddPreload("Profile", "active = ?", true)
	assert.Len(t, c.scopeClosures, 2)
}

func TestCriteria_GroupHavingJoins(t *testing.T) {
	c := NewCriteria()
	c.Group("category_id")
	c.Having("COUNT(*) > ?", 5)
	c.Joins("LEFT JOIN categories ON categories.id = products.category_id")

	assert.Equal(t, "category_id", c.group)
	assert.Len(t, c.scopeClosures, 2) // Having and Joins
}

func TestCriteria_GetLimitWithOffset(t *testing.T) {
	// 测试 offset 优先的情况
	c := NewCriteria().Offset(10).Limit(5)
	assert.Equal(t, 10, c.GetOffset())
	assert.Equal(t, 5, c.GetLimit())

	// 测试 page/per_page 计算 offset
	c2 := NewCriteria().Page(3).PerPage(10)
	assert.Equal(t, 20, c2.GetOffset()) // (3-1) * 10 = 20
	assert.Equal(t, 10, c2.GetLimit())
}

func TestCriteria_ExtractCriteriaWithPointer(t *testing.T) {
	type SearchReq struct {
		Name string `criteria:"name:eq"`
		Age  int    `criteria:"age:gt"`
	}

	req := &SearchReq{Name: "John", Age: 18}
	c, err := ExtractCriteria(req)
	assert.NoError(t, err)
	assert.NotNil(t, c)
	assert.Len(t, c.scopeClosures, 2)
}

func TestCriteria_ExtractCriteriaWithMultipleFields(t *testing.T) {
	type SearchReq struct {
		Keyword string `criteria:"title,content:like"`
	}

	req := SearchReq{Keyword: "test"}
	c, err := ExtractCriteria(req)
	assert.NoError(t, err)
	assert.NotNil(t, c)
	// keyword 会在 title 和 content 两个字段上执行 like
	assert.Len(t, c.scopeClosures, 1) // GroupOr creates one scope closure
}

func TestCriteria_ExtractCriteriaRejectsUnsafeSort(t *testing.T) {
	type SearchReq struct {
		Sort string `criteria:"sort:sort"`
	}

	_, err := ExtractCriteria(SearchReq{Sort: "name desc;drop table users"})
	assert.Error(t, err)
	if err != nil {
		assert.True(t, strings.Contains(err.Error(), "sort"))
	}
}

func TestCriteria_WhereBetweenQuotesReservedWord(t *testing.T) {
	c := NewCriteria()
	c.WhereBetween("order", 1, 10)

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	assert.NoError(t, err)

	tx := c.scopeClosures[0](db.Session(&gorm.Session{DryRun: true}).Model(&TestModel{})).Find(&[]TestModel{})
	assert.Contains(t, tx.Statement.SQL.String(), "`order` BETWEEN")
}
