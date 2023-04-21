package storeit

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestStruct struct {
	ID        int    `criteria:"id:eq"`
	Keywords  string `criteria:"name,nickname:like"`
	Keywords2 string `criteria:"a,b:llike"`
	Keywords3 string `criteria:"c:rlike"`
	Mobile    string `criteria:"mobile:eq"`
	CreatedAt string `criteria:"created_at:gte"`
	UpdatedAt string `criteria:"updated_at:lte"`
	Page      int    `criteria:"-:page"`
	PerPage   int    `criteria:"-:per_page"`
	Limit     int    `criteria:"-:limit"`
	Offset    int    `criteria:"-:offset"`
	Sort      string `criteria:"-:sort"`
}

func TestExtractCriteria(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expected    *Criteria
		expectedErr error
	}{
		{
			name: "empty",
			input: struct {
				ID   int
				Name string
			}{},
			expected:    &Criteria{},
			expectedErr: nil,
		},
		{
			name: "valid",
			input: TestStruct{
				ID:        1,
				Keywords:  "john",
				Keywords2: "week",
				Keywords3: "day",
				Mobile:    "1234t789",
				CreatedAt: "2022-01-01",
				UpdatedAt: "2022-02-01",
				Page:      2,
				PerPage:   10,
				Limit:     10,
				Offset:    10,
				Sort:      "name+,age-",
			},
			expected: &Criteria{
				whereConditions: []conditionSpec{
					{
						query: "id = ?",
						args:  []any{1},
					},
					{
						query: "c like ?",
						args:  []any{"day%"},
					},
					{
						query: "mobile = ?",
						args:  []any{"1234t789"},
					},
					{
						query: "created_at >= ?",
						args:  []any{"2022-01-01"},
					},
					{
						query: "updated_at <= ?",
						args:  []any{"2022-02-01"},
					},
				},
				groupOrConditions: []groupConditionSpec{
					[]conditionSpec{
						{
							query: "name like ?",
							args:  []any{"%john%"},
						},
						{
							query: "nickname like ?",
							args:  []any{"%john%"},
						},
					},
					[]conditionSpec{
						{
							query: "a like ?",
							args:  []any{"%week"},
						},
						{
							query: "b like ?",
							args:  []any{"%week"},
						},
					},
				},
				orders: []string{"name", "age DESC"},
				limit:  10,
				offset: 10,
				page:   2,
			},
			expectedErr: nil,
		},
		{
			name: "invalid source",
			input: map[string]string{
				"id":   "1",
				"name": "john",
			},
			expected:    nil,
			expectedErr: errors.New("extract source type must be a Struct"),
		},
		{
			name: "invalid tag",
			input: struct {
				ID   int
				Name string `criteria:"name"`
			}{
				ID:   1,
				Name: "john",
			},
			expected:    nil,
			expectedErr: errors.New("criteria condition tag error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ExtractCriteria(tt.input)
			if tt.expectedErr == nil {
				assert.True(t, reflect.DeepEqual(tt.expected, actual))
			} else {
				assert.Equal(t, tt.expectedErr, err)
			}
		})
	}
}

func TestCriteriaMethods(t *testing.T) {
	criteria := &Criteria{}
	assert.Equal(t, criteria, criteria.Where("id = ?", 1))
	assert.Equal(t, criteria, criteria.WhereNot("id = ?", 1))
	assert.Equal(t, criteria, criteria.OrWhere("id = ?", 1))
	assert.Equal(t, criteria, criteria.Having("count(*) > ?", 1))
	assert.Equal(t, criteria, criteria.Joins("join users on users.id =user_id"))
	criteria.Order("name", false)
	assert.Equal(t, []string{"name"}, criteria.orders)
	criteria.Order("age", true)
	assert.Equal(t, []string{"name", "age DESC"}, criteria.orders)
	criteria.unsetOrder()
	assert.Empty(t, criteria.orders)
	criteria.Limit(10)
	assert.Equal(t, 10, criteria.GetLimit())
	criteria.Offset(5)
	assert.Equal(t, 5, criteria.GetOffset())
	criteria.Page(2)
	assert.Equal(t, 2, criteria.GetPage())
	criteria.PerPage(20)
	assert.Equal(t, 20, criteria.GetPerPage())
	assert.Equal(t, criteria, criteria.Page(3))
	assert.Equal(t, 3, criteria.GetPage())
	assert.Equal(t, criteria, criteria.Page(-1))
	assert.Equal(t, 1, criteria.GetPage())
	assert.Equal(t, criteria, criteria.Offset(20))
	assert.Equal(t, 20, criteria.offset)
	assert.Equal(t, criteria, criteria.Group("name"))
	assert.Equal(t, "name", criteria.group)
	criteria.unsetOrder()
	assert.Nil(t, criteria.orders)
	criteria.unsetLimit()
	assert.Equal(t, 0, criteria.limit)
	assert.Equal(t, 0, criteria.offset)
}
