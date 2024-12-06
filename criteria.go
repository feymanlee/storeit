package storeit

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"golang.org/x/exp/slices"
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
	query any
	args  []any
}

type groupConditionSpec []conditionSpec

type joinSpec struct {
	query string
	args  []any
}
type havingSpec struct {
	query any
	args  []any
}

type Criteria struct {
	whereConditions   []conditionSpec
	groupOrConditions []groupConditionSpec
	orConditions      []conditionSpec
	notConditions     []conditionSpec
	havingConditions  []havingSpec
	joinConditions    []joinSpec
	orders            []string
	preloads          []preloadEntry
	limit             int
	offset            int
	group             string
	page              int
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

// ExtractCriteria 从结构体导出 Criteria
func ExtractCriteria(source any) (*Criteria, error) {
	if source == nil {
		return nil, errors.New("empty source")
	}
	t := reflect.TypeOf(source)
	if t.Kind() != reflect.Struct {
		return nil, errors.New("extract source type must be a Struct")
	}
	v := reflect.ValueOf(source)
	var criteria = Criteria{}
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
			value, err := AnyToInt(fieldValue)
			if err != nil {
				return nil, err
			}
			criteria.PerPage(value)
		case criteriaPage:
			value, err := AnyToInt(fieldValue)
			if err != nil {
				return nil, err
			}
			criteria.Page(value)
		case criteriaOffset:
			value, err := AnyToInt(fieldValue)
			if err != nil {
				return nil, err
			}
			criteria.Offset(value)
		case criteriaLimit:
			value, err := AnyToInt(fieldValue)
			if err != nil {
				return nil, err
			}
			criteria.Limit(value)
		case criteriaSort:
			value, err := AnyToString(fieldValue)
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
				if wc.query != nil {
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
			if wc.query != nil {
				criteria.Where(wc.query, wc.args...)
			}
		}
	}
	return &criteria, nil
}

func (c *Criteria) Where(query any, values ...any) *Criteria {
	c.whereConditions = append(c.whereConditions, conditionSpec{query: query, args: values})
	return c
}

func (c *Criteria) WhereNot(query any, values ...any) *Criteria {
	c.notConditions = append(c.notConditions, conditionSpec{query: query, args: values})
	return c
}

func (c *Criteria) WhereIsNull(field string) *Criteria {
	return c.Where(field + " IS NULL")
}

func (c *Criteria) WhereNotNull(field string) *Criteria {
	return c.Where(field + " IS NOT NULL")
}

func (c *Criteria) WhereIn(field string, values any) *Criteria {
	return c.Where(field+" IN ?", values)
}

func (c *Criteria) WhereNotIn(field string, values any) *Criteria {
	return c.Where(field+" NOT IN ?", values)
}

func (c *Criteria) WhereStartWith(field string, value string) *Criteria {
	return c.Where(field+" LIKE ?", "%"+value)
}

func (c *Criteria) WhereEndWith(field string, value string) *Criteria {
	return c.Where(field+" LIKE ?", value+"%")
}

func (c *Criteria) WhereContains(field string, value string) *Criteria {
	return c.Where(field+" LIKE ?", "%"+value+"%")
}

func (c *Criteria) WhereBetween(field string, start, end any) *Criteria {
	return c.Where(field+" BETWEEN ? AND ?", start, end)
}

func (c *Criteria) GroupOr(group groupConditionSpec) *Criteria {
	c.groupOrConditions = append(c.groupOrConditions, group)
	return c
}

func (c *Criteria) OrWhere(query any, values ...any) *Criteria {
	c.orConditions = append(c.orConditions, conditionSpec{query: query, args: values})
	return c
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
	c.havingConditions = append(c.havingConditions, havingSpec{query: query, args: values})
	return c
}

func (c *Criteria) Joins(query string, values ...any) *Criteria {
	c.joinConditions = append(c.joinConditions, joinSpec{query: query, args: values})
	return c
}

func (c *Criteria) AddPreload(name string, args ...any) *Criteria {
	c.preloads = append(c.preloads, preloadEntry{
		name: name,
		args: args,
	})
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

func (c *Criteria) buildConditionSpec(criteriaOperator string, field string, fieldValue any) (cond conditionSpec, err error) {
	if operator, ok := conditionMapping[criteriaOperator]; ok {
		cond.query = fmt.Sprintf("%s %s ?", field, operator)
		cond.args = []any{fieldValue}
	} else if slices.Contains(valueStringOperator, criteriaOperator) {
		var value string
		value, err = AnyToString(fieldValue)
		if err != nil {
			return
		}
		switch criteriaOperator {
		case criteriaLike:
			cond.query = fmt.Sprintf("%s like ?", field)
			cond.args = []any{"%" + value + "%"}
		case criteriaLLike:
			cond.query = fmt.Sprintf("%s like ?", field)
			cond.args = []any{"%" + value}
		case criteriaRLike:
			cond.query = fmt.Sprintf("%s like ?", field)
			cond.args = []any{value + "%"}
		}
	}
	return
}
