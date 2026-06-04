// Package storeit provides a generic repository pattern wrapper around GORM.
// This file contains the Criteria builder for dynamic query filtering.
package storeit

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/spf13/cast"
	"gorm.io/gorm"
)

// Constants for criteria operators and special fields.
const (
	criteriaLike    = "like"     // LIKE "%value%" - contains pattern
	criteriaLLike   = "llike"    // LIKE "%value" - ends with pattern
	criteriaRLike   = "rlike"    // LIKE "value%" - starts with pattern
	criteriaSort    = "sort"     // Sort/order by field
	criteriaPerPage = "per_page" // Items per page for pagination
	criteriaPage    = "page"     // Page number for pagination
	criteriaOffset  = "offset"   // Offset for pagination
	criteriaLimit   = "limit"    // Limit/maximum number of results
	criteriaNotIn   = "notin"    // NOT IN list
	criteriaIsNull  = "isnull"   // IS NULL check
	criteriaNotNull = "notnull"  // IS NOT NULL check
	criteriaBetween = "between"  // BETWEEN range
)

// conditionSpec represents a single WHERE condition with query string and arguments.
type conditionSpec struct {
	query string // The SQL query fragment (e.g., "name = ?")
	args  []any  // Arguments to substitute into the query
}

// groupConditionSpec represents a group of conditions combined with OR.
type groupConditionSpec []conditionSpec

// Criteria is a query builder for dynamic filtering.
// It supports method chaining for building complex queries with
// conditions, ordering, pagination, and grouping.
type Criteria struct {
	scopeClosures []gormClosure // Custom GORM scope closures
	orders        []string      // ORDER BY clauses
	limit         int           // LIMIT value
	offset        int           // OFFSET value
	group         string        // GROUP BY clause
	page          int           // Current page number (1-based)
}

// conditionMapping maps operator names to SQL operators.
// These are used in struct tag-based criteria extraction.
var conditionMapping = map[string]string{
	"eq":  "=",  // Equal
	"neq": "<>", // Not equal
	"gt":  ">",  // Greater than
	"gte": ">=", // Greater than or equal
	"lt":  "<",  // Less than
	"lte": "<=", // Less than or equal
	"in":  "IN", // In list
}

var safeIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)

// NewCriteria creates a new empty Criteria instance.
//
// Example:
//
//	criteria := storeit.NewCriteria().Where("status = ?", "active").Limit(10)
func NewCriteria() *Criteria {
	return &Criteria{}
}

// ExtractCriteria extracts criteria from a struct using "criteria" tags.
//
// Tag syntax: `criteria:"field,operator"` or `criteria:"-:operator"` for meta fields
//
// Supported operators:
//   - eq, neq, gt, gte, lt, lte, in: Comparison operators
//   - like, llike, rlike: LIKE patterns (contains, ends with, starts with)
//   - page, per_page, offset, limit: Pagination
//   - sort: Sorting (use "+" suffix for ASC, "-" suffix for DESC)
//
// Example struct:
//
//	type SearchRequest struct {
//	    ID      int    `form:"id" criteria:"id,eq"`
//	    Keyword string `form:"keyword" criteria:"name,email:like"` // OR across fields
//	    Name    string `form:"name" criteria:"name:llike"`         // LIKE "%value"
//	    Status  string `form:"status" criteria:"status:eq"`
//	    Page    int    `form:"page" criteria:"-:page"`
//	    Sorts   string `form:"sorts" criteria:"-:sort"`            // "name-,age+" = name DESC, age ASC
//	}
func ExtractCriteria(source any) (*Criteria, error) {
	if source == nil {
		return nil, errors.New("empty source")
	}

	v := reflect.ValueOf(source)
	// Handle pointer types
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

	// Preallocate capacity to reduce memory allocations
	var criteria = Criteria{
		scopeClosures: make([]gormClosure, 0, t.NumField()),
		orders:        make([]string, 0, t.NumField()),
	}

	for i := 0; i < v.NumField(); i++ {
		sf := t.Field(i)
		// Skip if field value is zero value
		if v.FieldByName(sf.Name).IsZero() {
			continue
		}
		// Skip if no criteria tag
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
		// Handle pagination and ordering
		switch criteriaOperator {
		case criteriaPerPage:
			value, err := cast.ToIntE(fieldValue)
			if err != nil {
				return nil, err
			}
			criteria.PerPage(value)
			continue
		case criteriaPage:
			value, err := cast.ToIntE(fieldValue)
			if err != nil {
				return nil, err
			}
			criteria.Page(value)
			continue
		case criteriaOffset:
			value, err := cast.ToIntE(fieldValue)
			if err != nil {
				return nil, err
			}
			criteria.Offset(value)
			continue
		case criteriaLimit:
			value, err := cast.ToIntE(fieldValue)
			if err != nil {
				return nil, err
			}
			criteria.Limit(value)
			continue
		case criteriaSort:
			value, err := cast.ToStringE(fieldValue)
			if err != nil {
				return nil, fmt.Errorf("parse sort field %s: %w", sf.Name, err)
			}
			orders := strings.Split(value, ",")
			for _, order := range orders {
				sortField := strings.TrimSpace(strings.TrimRight(order, "+-"))
				if !isSafeIdentifier(sortField) {
					return nil, fmt.Errorf("invalid sort field %q for %s", sortField, sf.Name)
				}
				criteria.Order(sortField, strings.HasSuffix(order, "-"))
			}
			continue
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

// Where adds a WHERE condition to the criteria.
// Multiple Where calls are combined with AND.
//
// Example:
//
//	criteria.Where("status = ?", "active").Where("age > ?", 18)
func (c *Criteria) Where(query any, values ...any) *Criteria {
	return c.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
		return tx.Where(query, values...)
	})
}

// WhereGt adds a WHERE condition for "field > value".
func (c *Criteria) WhereGt(field string, value any) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" > ?", value)
}

// WhereGte adds a WHERE condition for "field >= value".
func (c *Criteria) WhereGte(field string, value any) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" >= ?", value)
}

// WhereLte adds a WHERE condition for "field <= value".
func (c *Criteria) WhereLte(field string, value any) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" <= ?", value)
}

// WhereLt adds a WHERE condition for "field < value".
func (c *Criteria) WhereLt(field string, value any) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" < ?", value)
}

// WhereNeq adds a WHERE condition for "field <> value" (not equal).
func (c *Criteria) WhereNeq(field string, value any) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" <> ?", value)
}

// buildConditionSpec builds a condition specification from the operator, field, and value.
// It uses QuoteReservedWord to protect field names that are MySQL reserved words.
func (c *Criteria) buildConditionSpec(criteriaOperator string, field string, fieldValue any) (cond conditionSpec, err error) {
	field = QuoteReservedWord(field)
	cond = conditionSpec{}
	if operator, ok := conditionMapping[criteriaOperator]; ok {
		cond.query = fmt.Sprintf("%s %s ?", field, operator)
		cond.args = []any{fieldValue}
		return
	}

	switch criteriaOperator {
	case criteriaLike, criteriaLLike, criteriaRLike:
		var value string
		value, err = cast.ToStringE(fieldValue)
		if err != nil {
			return
		}
		cond = buildLikeCondition(field, value, criteriaOperator)
	case criteriaNotIn:
		cond.query = fmt.Sprintf("%s NOT IN ?", field)
		cond.args = []any{fieldValue}
	case criteriaIsNull:
		cond.query = fmt.Sprintf("%s IS NULL", field)
	case criteriaNotNull:
		cond.query = fmt.Sprintf("%s IS NOT NULL", field)
	case criteriaBetween:
		values := reflect.ValueOf(fieldValue)
		if values.Kind() != reflect.Array && values.Kind() != reflect.Slice {
			return cond, fmt.Errorf("between field %s must be an array or slice", field)
		}
		if values.Len() != 2 {
			return cond, fmt.Errorf("between field %s must contain exactly 2 values", field)
		}
		cond.query = fmt.Sprintf("%s BETWEEN ? AND ?", field)
		cond.args = []any{values.Index(0).Interface(), values.Index(1).Interface()}
	}
	return
}

// buildLikeCondition is a helper function for building LIKE conditions.
// It reduces code duplication for different LIKE patterns.
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

// GroupOr adds a group of OR conditions to the criteria.
// All conditions in the group are combined with OR and wrapped in parentheses.
//
// Example:
//
//	criteria.GroupOr([]conditionSpec{
//	    {query: "name = ?", args: []any{"John"}},
//	    {query: "name = ?", args: []any{"Jane"}},
//	})
//
// Results in: (name = 'John' OR name = 'Jane')
func (c *Criteria) GroupOr(group groupConditionSpec) *Criteria {
	if len(group) == 0 {
		return c // Return unchanged if group is empty
	}
	return c.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
		sub := tx.Session(&gorm.Session{NewDB: true})
		for _, cond := range group {
			sub = sub.Or(cond.query, cond.args...)
		}
		return tx.Where(sub)
	})
}

// WhereNot adds a NOT condition to the criteria.
//
// Example:
//
//	criteria.WhereNot("status", "deleted")
func (c *Criteria) WhereNot(query any, values ...any) *Criteria {
	return c.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
		return tx.Not(query, values...)
	})
}

// WhereIsNull adds a WHERE condition for "field IS NULL".
func (c *Criteria) WhereIsNull(field string) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field + " IS NULL")
}

// WhereNotNull adds a WHERE condition for "field IS NOT NULL".
func (c *Criteria) WhereNotNull(field string) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field + " IS NOT NULL")
}

// WhereIn adds a WHERE condition for "field IN (values)".
//
// Example:
//
//	criteria.WhereIn("status", []string{"active", "pending"})
func (c *Criteria) WhereIn(field string, values any) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" IN ?", values)
}

// WhereNotIn adds a WHERE condition for "field NOT IN (values)".
//
// Example:
//
//	criteria.WhereNotIn("status", []string{"deleted", "banned"})
func (c *Criteria) WhereNotIn(field string, values any) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" NOT IN ?", values)
}

// WhereStartWith adds a WHERE condition for "field LIKE 'value%'".
func (c *Criteria) WhereStartWith(field string, value string) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" LIKE ?", value+"%")
}

// WhereEndWith adds a WHERE condition for "field LIKE '%value'".
func (c *Criteria) WhereEndWith(field string, value string) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" LIKE ?", "%"+value)
}

// WhereContains adds a WHERE condition for "field LIKE '%value%'".
func (c *Criteria) WhereContains(field string, value string) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" LIKE ?", "%"+value+"%")
}

// WhereBetween adds a WHERE condition for "field BETWEEN start AND end".
//
// Example:
//
//	criteria.WhereBetween("age", 18, 65)
func (c *Criteria) WhereBetween(field string, start, end any) *Criteria {
	field = QuoteReservedWord(field)
	return c.Where(field+" BETWEEN ? AND ?", start, end)
}

// OrWhere adds an OR condition to the criteria.
//
// Example:
//
//	criteria.Where("status = ?", "active").OrWhere("role = ?", "admin")
func (c *Criteria) OrWhere(query any, values ...any) *Criteria {
	return c.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
		return tx.Or(query, values...)
	})
}

// Order adds an ORDER BY clause to the criteria.
// Set isDescending to true for DESC order, false for ASC.
//
// Example:
//
//	criteria.Order("created_at", true)  // ORDER BY created_at DESC
//	criteria.Order("name", false)       // ORDER BY name ASC
func (c *Criteria) Order(value string, isDescending bool) *Criteria {
	orderStatement := QuoteReservedWord(value)
	if isDescending {
		orderStatement = fmt.Sprintf("%s DESC", orderStatement)
	}

	c.orders = append(c.orders, orderStatement)
	return c
}

// OrderDesc adds a descending ORDER BY clause.
//
// Example:
//
//	criteria.OrderDesc("created_at")
func (c *Criteria) OrderDesc(value string) *Criteria {
	c.Order(value, true)
	return c
}

// OrderAsc adds an ascending ORDER BY clause.
//
// Example:
//
//	criteria.OrderAsc("name")
func (c *Criteria) OrderAsc(value string) *Criteria {
	c.Order(value, false)
	return c
}

// Limit sets the maximum number of records to return.
//
// Example:
//
//	criteria.Limit(100)
func (c *Criteria) Limit(limit int) *Criteria {
	c.limit = limit
	return c
}

// Offset sets the number of records to skip.
//
// Example:
//
//	criteria.Offset(20)
func (c *Criteria) Offset(offset int) *Criteria {
	c.offset = offset
	return c
}

// Page sets the current page number for pagination.
// Page numbers are 1-based (first page is 1, not 0).
//
// Example:
//
//	criteria.Page(2).PerPage(50)
func (c *Criteria) Page(page int) *Criteria {
	if page < 1 {
		page = 1
	}
	c.page = page
	return c
}

// PerPage sets the number of items per page for pagination.
// This is an alias for Limit.
//
// Example:
//
//	criteria.Page(1).PerPage(20)
func (c *Criteria) PerPage(perPage int) *Criteria {
	c.limit = perPage
	return c
}

// Group sets the GROUP BY clause.
//
// Example:
//
//	criteria.Group("department")
func (c *Criteria) Group(query string) *Criteria {
	c.group = query
	return c
}

// Having adds a HAVING clause for filtering grouped results.
//
// Example:
//
//	criteria.Group("department").Having("COUNT(*) > ?", 5)
func (c *Criteria) Having(query any, values ...any) *Criteria {
	return c.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
		return tx.Having(query, values...)
	})
}

// Joins adds a JOIN clause to the query.
//
// Example:
//
//	criteria.Joins("LEFT JOIN orders ON users.id = orders.user_id")
func (c *Criteria) Joins(query string, values ...any) *Criteria {
	return c.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
		return tx.Joins(query, values...)
	})
}

// AddPreload adds a preload clause for eager loading associations.
//
// Example:
//
//	criteria.AddPreload("Orders").AddPreload("Orders.Items")
func (c *Criteria) AddPreload(name string, args ...any) *Criteria {
	return c.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
		return tx.Preload(name, args...)
	})
}

// ScopeClosure adds a custom GORM scope closure to the criteria.
//
// Example:
//
//	criteria.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
//	    return tx.Where("created_at > ?", time.Now().AddDate(0, -1, 0))
//	})
func (c *Criteria) ScopeClosure(closure gormClosure) *Criteria {
	c.scopeClosures = append(c.scopeClosures, closure)
	return c
}

// GetPage returns the current page number.
func (c *Criteria) GetPage() int {
	return c.page
}

// GetPerPage returns the number of items per page.
func (c *Criteria) GetPerPage() int {
	return c.limit
}

// GetOffset calculates and returns the offset for pagination.
// If offset is explicitly set, returns that value.
// Otherwise, calculates from page and limit.
func (c *Criteria) GetOffset() int {
	if c.offset > 0 {
		return c.offset
	}
	return c.GetLimit() * (c.GetPage() - 1)
}

// GetLimit returns the limit (maximum number of records).
func (c *Criteria) GetLimit() int {
	return c.limit
}

// unsetOrder clears all ORDER BY clauses.
// Used internally for count queries where ordering is not needed.
func (c *Criteria) unsetOrder() {
	c.orders = nil
}

// unsetLimit clears the limit and offset values.
// Used internally for count queries where limits are not needed.
func (c *Criteria) unsetLimit() {
	c.limit = 0
	c.offset = 0
}

func isSafeIdentifier(field string) bool {
	if field == "" {
		return false
	}
	return safeIdentifierPattern.MatchString(field)
}
