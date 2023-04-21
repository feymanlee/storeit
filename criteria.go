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
}

var valueStringOperator = []string{criteriaLike, criteriaLLike, criteriaRLike, criteriaSort}

func NewCriteria() *Criteria {
	return &Criteria{}
}

// ExtractCriteria 从结构体导出 Criteria
func ExtractCriteria(source any) (*Criteria, error) {
	if source == nil {
		return nil, nil
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

func (s *Criteria) Where(query any, values ...any) *Criteria {
	s.whereConditions = append(s.whereConditions, conditionSpec{query: query, args: values})
	return s
}

func (s *Criteria) WhereNot(query any, values ...any) *Criteria {
	s.notConditions = append(s.notConditions, conditionSpec{query: query, args: values})
	return s
}

func (s *Criteria) WhereIsNull(field string) *Criteria {
	return s.Where("? IS NULL", field)
}

func (s *Criteria) WhereNotNull(field string) *Criteria {
	return s.Where("? IS NOT NULL", field)
}

func (s *Criteria) WhereNotIn(field string, values any) *Criteria {
	return s.Where(field+" IN ?", values)
}

func (s *Criteria) WhereIn(field string, values any) *Criteria {
	return s.Where(field+" NOT IN ?", values)
}

func (s *Criteria) WhereBetween(field string, start, end any) *Criteria {
	return s.Where(field+" BETWEEN ? AND ?", start, end)
}

func (s *Criteria) GroupOr(group groupConditionSpec) *Criteria {
	s.groupOrConditions = append(s.groupOrConditions, group)
	return s
}

func (s *Criteria) OrWhere(query any, values ...any) *Criteria {
	s.orConditions = append(s.orConditions, conditionSpec{query: query, args: values})
	return s
}

func (s *Criteria) Order(value string, reorder bool) *Criteria {
	if s.orders == nil {
		s.orders = []string{}
	}

	if reorder {
		s.orders = append(s.orders, value+" DESC")
	} else {
		s.orders = append(s.orders, value)
	}
	return s
}

func (s *Criteria) Limit(limit int) *Criteria {
	s.limit = limit
	return s
}

func (s *Criteria) Offset(offset int) *Criteria {
	s.offset = offset
	return s
}

func (s *Criteria) Page(page int) *Criteria {
	if page < 1 {
		page = 1
	}
	s.page = page
	return s
}

func (s *Criteria) PerPage(perPage int) *Criteria {
	s.limit = perPage
	return s
}

func (s *Criteria) Group(query string) *Criteria {
	s.group = query
	return s
}

func (s *Criteria) Having(query any, values ...any) *Criteria {
	s.havingConditions = append(s.havingConditions, havingSpec{query: query, args: values})
	return s
}

func (s *Criteria) Joins(query string, values ...any) *Criteria {
	s.joinConditions = append(s.joinConditions, joinSpec{query: query, args: values})
	return s
}

func (s *Criteria) GetPage() int {
	return s.page
}

func (s *Criteria) GetPerPage() int {
	return s.limit
}

func (s *Criteria) GetOffset() int {
	if s.offset > 0 {
		return s.offset
	}
	return s.GetLimit() * (s.GetPage() - 1)
}

func (s *Criteria) GetLimit() int {
	return s.limit
}

func (s *Criteria) unsetOrder() {
	s.orders = nil
}

func (s *Criteria) unsetLimit() {
	s.limit = 0
	s.offset = 0
}

func (s *Criteria) buildConditionSpec(criteriaOperator string, field string, fieldValue any) (cond conditionSpec, err error) {
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
