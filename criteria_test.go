package storeit

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.Len(t, c.scopeClosures, 2)
}

func TestCriteria_WhereGtGteLtLte(t *testing.T) {
	c := NewCriteria()
	c.WhereGt("age", 10)
	c.WhereGte("age", 11)
	c.WhereLt("age", 20)
	c.WhereLte("age", 21)
	assert.Len(t, c.scopeClosures, 4)
}

func TestCriteria_WhereNotAndNull(t *testing.T) {
	c := NewCriteria()
	c.WhereNot("name", "foo")
	c.WhereIsNull("email")
	c.WhereNotNull("email")
	assert.Len(t, c.scopeClosures, 3)
}

func TestCriteria_WhereInNotIn(t *testing.T) {
	c := NewCriteria()
	c.WhereIn("status", []string{"a", "b"})
	c.WhereNotIn("status", []string{"c"})
	assert.Len(t, c.scopeClosures, 2)
}

func TestCriteria_WhereStartEndContainsBetween(t *testing.T) {
	c := NewCriteria()
	c.WhereStartWith("name", "A")
	c.WhereEndWith("name", "Z")
	c.WhereContains("desc", "foo")
	c.WhereBetween("age", 1, 10)
	assert.Len(t, c.scopeClosures, 4)
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
	c.AddPreload("User")
	assert.Equal(t, "status", c.group)
	assert.Len(t, c.scopeClosures, 3)
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
	assert.Len(t, c.scopeClosures, 1)

	// 空组
	c2 := NewCriteria()
	c2.GroupOr(groupConditionSpec{})
	assert.Len(t, c2.scopeClosures, 0)
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
	assert.Empty(t, c.scopeClosures)
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
	assert.Len(t, c.scopeClosures, 1)
}

func TestCriteria_OrderReservedWord(t *testing.T) {
	c := NewCriteria()
	c.Order("order", false)
	assert.Contains(t, c.orders[0], "`order`")
}
