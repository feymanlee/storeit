package storeit

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/cast"
	"golang.org/x/exp/slices"
	"gorm.io/gorm"
)

const (
	criteriaLike    = "like"
	criteriaLLike   = "llike"
	criteriaRLike   = "rlike"
	criteriaSort    = "sort"
	criteriaPerPage = "per_page"
	criteriaPage    = "page"
	criteriaOffset  = "offset"
	criteriaLimit   = "limit"
)

type conditionSpec struct {
	query string
	args  []any
}

type groupConditionSpec []conditionSpec

type Criteria struct {
	scopeClosures []gormClosure
	orders        []string
	limit         int
	offset        int
	group         string
	page          int
}

var conditionMapping = map[string]string{
	"eq":  "=",
	"neq": "<>",
	"gt":  ">",
	"gte": ">=",
	"lt":  "<",
	"lte": "<=",
	"in":  "IN",
}

var valueStringOperator = []string{criteriaLike, criteriaLLike, criteriaRLike, criteriaSort}

func NewCriteria() *Criteria {
	return &Criteria{}
}

func ExtractCriteria(source any) (*Criteria, error) {
	if source == nil {
		return nil, errors.New("empty source")
	}

	v := reflect.ValueOf(source)
	// 处理指针类型
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, errors.New("nil pointer source")
		}
		v = v.Elem()
	}

	t := v.Type()
	if t.Kind() != reflect.Struct {
		return nil, errors.New("extract source type must be a Struct")
	}

	// 预分配容量，减少内存分配
	var criteria = Criteria{
		scopeClosures: make([]gormClosure, 0, t.NumField()),
		orders:        make([]string, 0, t.NumField()),
	}

	for i := 0; i < v.NumField(); i++ {
		sf := t.Field(i)
		// if field value is zero value skip
		if v.FieldByName(sf.Name).IsZero() {
			continue
		}
		// if not criteria tag skip
		criteriaTag := sf.Tag.Get("criteria")
		if criteriaTag == "" {
			continue
		}
		criteriaOptions := strings.Split(criteriaTag, ":")
		if len(criteriaOptions) != 2 {
			return nil, errors.New("criteria condition tag error")
		}
		criteriaOperator := criteriaOptions[1]
		fieldValue := v.FieldByName(sf.Name).Interface()
		// 处理分页和 order
		switch criteriaOperator {
		case criteriaPerPage:
			value, err := cast.ToIntE(fieldValue)
			if err != nil {
				return nil, err
			}
			criteria.PerPage(value)
		case criteriaPage:
			value, err := cast.ToIntE(fieldValue)
			if err != nil {
				return nil, err
			}
			criteria.Page(value)
		case criteriaOffset:
			value, err := cast.ToIntE(fieldValue)
			if err != nil {
				return nil, err
			}
			criteria.Offset(value)
		case criteriaLimit:
			value, err := cast.ToIntE(fieldValue)
			if err != nil {
				return nil, err
			}
			criteria.Limit(value)
		case criteriaSort:
			value, err := cast.ToStringE(fieldValue)
			if err != nil {
				return nil, err
			}
			orders := strings.Split(value, ",")
			for _, order := range orders {
				criteria.Order(strings.TrimSpace(strings.TrimRight(order, "+-")), strings.HasSuffix(order, "-"))
			}
		}
		fields := strings.Split(criteriaOptions[0], ",")
		if len(fields) > 1 {
			groupSpec := make(groupConditionSpec, 0, len(fields))
			for _, field := range fields {
				wc, err := criteria.buildConditionSpec(criteriaOperator, field, fieldValue)
				if err != nil {
					return nil, err
				}
				if wc.query != "" {
					groupSpec = append(groupSpec, wc)
				}
			}
			if len(groupSpec) > 0 {
				criteria.GroupOr(groupSpec)
			}
		} else {
			wc, err := criteria.buildConditionSpec(criteriaOperator, criteriaOptions[0], fieldValue)
			if err != nil {
				return nil, err
			}
			if wc.query != "" {
				criteria.Where(wc.query, wc.args...)
			}
		}
	}
	return &criteria, nil
}

func (c *Criteria) Where(query any, values ...any) *Criteria {
	return c.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
		return tx.Where(query, values...)
	})
}

func (c *Criteria) WhereGt(field string, value any) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" > ?", value)
}

func (c *Criteria) WhereGte(field string, value any) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" >= ?", value)
}

func (c *Criteria) WhereLte(field string, value any) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" <= ?", value)
}

func (c *Criteria) WhereLt(field string, value any) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" < ?", value)
}

func (c *Criteria) WhereNeq(field string, value any) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" <> ?", value)
}

// 优化 buildConditionSpec 方法，使用 QuoteReservedWord 保护字段名
func (c *Criteria) buildConditionSpec(criteriaOperator string, field string, fieldValue any) (cond conditionSpec, err error) {
	field = QuoteReservedWord(field)
	cond = conditionSpec{}
	if operator, ok := conditionMapping[criteriaOperator]; ok {
		cond.query = fmt.Sprintf("%s %s ?", field, operator)
		cond.args = []any{fieldValue}
	} else if slices.Contains(valueStringOperator, criteriaOperator) {
		var value string
		value, err = cast.ToStringE(fieldValue)
		if err != nil {
			return
		}
		cond = buildLikeCondition(field, value, criteriaOperator)
	}
	return
}

// 添加一个辅助函数，用于构建 LIKE 条件，减少代码重复
func buildLikeCondition(field, value, likeType string) (cond conditionSpec) {
	field = QuoteReservedWord(field)
	cond.query = fmt.Sprintf("%s like ?", field)

	switch likeType {
	case criteriaLike:
		cond.args = []any{"%" + value + "%"}
	case criteriaLLike:
		cond.args = []any{"%" + value}
	case criteriaRLike:
		cond.args = []any{value + "%"}
	}

	return cond
}

func (c *Criteria) GroupOr(group groupConditionSpec) *Criteria {
	if len(group) == 0 {
		return c // 如果组为空，直接返回
	}
	return c.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
		sub := tx.Session(&gorm.Session{NewDB: true})
		for _, cond := range group {
			sub = sub.Or(cond.query, cond.args...)
		}
		return tx.Where(sub)
	})
}

func (c *Criteria) WhereNot(query any, values ...any) *Criteria {
	return c.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
		return tx.Not(query, values...)
	})
}

func (c *Criteria) WhereIsNull(field string) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field + " IS NULL")
}

func (c *Criteria) WhereNotNull(field string) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field + " IS NOT NULL")
}

func (c *Criteria) WhereIn(field string, values any) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" IN ?", values)
}

func (c *Criteria) WhereNotIn(field string, values any) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" NOT IN ?", values)
}

func (c *Criteria) WhereStartWith(field string, value string) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" LIKE ?", value+"%")
}

func (c *Criteria) WhereEndWith(field string, value string) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" LIKE ?", "%"+value)
}

func (c *Criteria) WhereContains(field string, value string) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" LIKE ?", "%"+value+"%")
}

func (c *Criteria) WhereBetween(field string, start, end any) *Criteria {
	return c.Where(field+" BETWEEN ? AND ?", start, end)
}

func (c *Criteria) OrWhere(query any, values ...any) *Criteria {
	return c.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
		return tx.Or(query, values...)
	})
}

func (c *Criteria) Order(value string, isDescending bool) *Criteria {
	orderStatement := QuoteReservedWord(value)
	if isDescending {
		orderStatement = fmt.Sprintf("%s DESC", orderStatement)
	}

	c.orders = append(c.orders, orderStatement)
	return c
}

func (c *Criteria) OrderDesc(value string) *Criteria {
	c.Order(value, true)
	return c
}

func (c *Criteria) OrderAsc(value string) *Criteria {
	c.Order(value, false)
	return c
}

func (c *Criteria) Limit(limit int) *Criteria {
	c.limit = limit
	return c
}

func (c *Criteria) Offset(offset int) *Criteria {
	c.offset = offset
	return c
}

func (c *Criteria) Page(page int) *Criteria {
	if page < 1 {
		page = 1
	}
	c.page = page
	return c
}

func (c *Criteria) PerPage(perPage int) *Criteria {
	c.limit = perPage
	return c
}

func (c *Criteria) Group(query string) *Criteria {
	c.group = query
	return c
}

func (c *Criteria) Having(query any, values ...any) *Criteria {
	return c.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
		return tx.Having(query, values...)
	})
}

func (c *Criteria) Joins(query string, values ...any) *Criteria {
	return c.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
		return tx.Joins(query, values...)
	})
}

func (c *Criteria) AddPreload(name string, args ...any) *Criteria {
	return c.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
		return tx.Preload(name, args...)
	})
}

func (c *Criteria) ScopeClosure(closure gormClosure) *Criteria {
	c.scopeClosures = append(c.scopeClosures, closure)
	return c
}

func (c *Criteria) GetPage() int {
	return c.page
}

func (c *Criteria) GetPerPage() int {
	return c.limit
}

func (c *Criteria) GetOffset() int {
	if c.offset > 0 {
		return c.offset
	}
	return c.GetLimit() * (c.GetPage() - 1)
}

func (c *Criteria) GetLimit() int {
	return c.limit
}

func (c *Criteria) unsetOrder() {
	c.orders = nil
}

func (c *Criteria) unsetLimit() {
	c.limit = 0
	c.offset = 0
}
