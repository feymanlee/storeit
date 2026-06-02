// Package storeit provides a generic repository pattern wrapper around GORM.
// It simplifies database operations by offering a fluent API for CRUD operations,
// pagination, and dynamic query building through struct tags.
package storeit

import (
	"context"
	"errors"
	"sync"

	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

var (
	ErrNilModels     = errors.New("models must not be nil")
	ErrNilCallback   = errors.New("callback must not be nil")
	ErrInvalidBatch  = errors.New("batch size must be greater than zero")
	ErrEmptyID       = errors.New("id is empty")
	ErrInvalidColumn = errors.New("column must not be empty")
)

// gormClosure is a function type that modifies a GORM DB instance.
// It is used for building query scopes and applying filters.
type gormClosure func(tx *gorm.DB) *gorm.DB

// Pagination represents a paginated query result.
// It contains metadata about the pagination state and the actual data items.
type Pagination[M any] struct {
	Total   int64 `json:"total"`    // Total number of records matching the query
	PerPage int   `json:"per_page"` // Number of items per page
	Page    int   `json:"page"`     // Current page number
	Items   []M   `json:"items"`    // Slice of model instances for the current page
}

// GormStore is a generic repository struct that wraps GORM operations.
// It provides a fluent API for building and executing database queries with
// support for column selection, hidden fields, scopes, and transactions.
//
// Type parameter M represents the model type this store operates on.
type GormStore[M interface{}] struct {
	tx            *gorm.DB      // Transaction context, if any
	db            *gorm.DB      // Underlying GORM database connection
	columns       []string      // Columns to select in queries
	hidden        []string      // Columns to omit from results
	scopeClosures []gormClosure // Query scope modifiers
	mu            sync.Mutex    // Mutex for thread-safe state operations
	unscoped      bool          // Whether to include soft-deleted records
}

// New creates a new GormStore instance for the given model type.
//
// Example:
//
//	store := storeit.New[User](db)
func New[M any](db *gorm.DB) *GormStore[M] {
	return &GormStore[M]{
		db: db,
	}
}

// SetTx sets the transaction context for the store.
// It returns a new cloned store instance with the transaction set.
// If tx is nil, returns the original store unchanged.
//
// Example:
//
//	store.SetTx(tx).Create(ctx, &user)
func (r *GormStore[M]) SetTx(tx *gorm.DB) *GormStore[M] {
	if tx == nil {
		return r
	}
	nr := r.onceClone()
	nr.tx = tx
	return nr
}

// Insert is an alias for Create. It creates a new record in the database.
//
// Example:
//
//	store.Insert(ctx, &user)
func (r *GormStore[M]) Insert(ctx context.Context, model *M) *gorm.DB {
	return r.Create(ctx, model)
}

// Unscoped returns a new store instance that includes soft-deleted records in queries.
// This temporarily disables GORM's soft delete feature.
//
// Example:
//
//	store.Unscoped().Find(ctx, criteria)
func (r *GormStore[M]) Unscoped() *GormStore[M] {
	nr := r.onceClone()
	nr.unscoped = true
	return nr
}

// WithTrashed controls whether soft-deleted records are included in queries.
// When with is true, soft-deleted records are included (same as Unscoped).
//
// Example:
//
//	store.WithTrashed(true).All(ctx)
func (r *GormStore[M]) WithTrashed(with bool) *GormStore[M] {
	nr := r.onceClone()
	nr.unscoped = with
	return nr
}

// Hidden specifies columns to omit from query results.
// It returns a new cloned store instance with the hidden fields set.
//
// Example:
//
//	store.Hidden([]string{"password", "secret_key"}).Find(ctx, criteria)
func (r *GormStore[M]) Hidden(fields []string) *GormStore[M] {
	return r.addHiddenColumns(fields)
}

// Emit is an alias for Hidden. It specifies columns to omit from query results.
func (r *GormStore[M]) Emit(fields []string) *GormStore[M] {
	nr := r.onceClone()
	return nr.Hidden(fields)
}

// Columns specifies which columns to select in queries.
// It returns a new cloned store instance with the columns set.
//
// Example:
//
//	store.Columns([]string{"id", "name", "email"}).Find(ctx, criteria)
func (r *GormStore[M]) Columns(fields []string) *GormStore[M] {
	return r.addColumns(fields)
}

// Create inserts a new record into the database.
// The model's primary key will be populated after successful creation.
//
// Example:
//
//	user := User{Name: "John", Email: "john@example.com"}
//	result := store.Create(ctx, &user)
func (r *GormStore[M]) Create(ctx context.Context, model *M) *gorm.DB {
	tx := r.present(ctx, nil).Create(model)
	r.reset()
	return tx
}

// Creates inserts multiple records into the database in a single operation.
//
// Example:
//
//	users := []User{{Name: "John"}, {Name: "Jane"}}
//	result := store.Creates(ctx, users)
func (r *GormStore[M]) Creates(ctx context.Context, models []M) *gorm.DB {
	tx := r.present(ctx, nil).Create(&models)
	r.reset()
	return tx
}

// CreateInBatches inserts multiple records in batches of the specified size.
// This is useful for large datasets to avoid memory issues.
//
// Example:
//
//	users := make([]User, 10000)
//	result := store.CreateInBatches(ctx, users, 1000)
func (r *GormStore[M]) CreateInBatches(ctx context.Context, models []M, batchSize int) *gorm.DB {
	tx := r.present(ctx, nil).CreateInBatches(&models, batchSize)
	r.reset()
	return tx
}

// Delete deletes records matching the provided model's primary key or conditions.
// For soft delete models, this sets the deleted_at timestamp.
//
// Example:
//
//	store.Delete(ctx, &user)
func (r *GormStore[M]) Delete(ctx context.Context, model *M) *gorm.DB {
	tx := r.present(ctx, nil).Delete(model)
	r.reset()
	return tx
}

// Deletes deletes all records matching the given criteria.
//
// Example:
//
//	criteria := storeit.NewCriteria().Where("status = ?", "inactive")
//	store.Deletes(ctx, criteria)
func (r *GormStore[M]) Deletes(ctx context.Context, criteria *Criteria) *gorm.DB {
	var model M
	tx := r.present(ctx, criteria).Delete(&model)
	r.reset()
	return tx
}

// DeleteById deletes a record by its primary key ID.
//
// Example:
//
//	store.DeleteById(ctx, 123)
func (r *GormStore[M]) DeleteById(ctx context.Context, id any) *gorm.DB {
	var model M
	tx := r.present(ctx, nil).Delete(&model, &id)
	r.reset()

	return tx
}

// Updates updates records matching the criteria with the given attributes.
// Attributes is a map or struct containing the fields to update.
//
// Example:
//
//	criteria := storeit.NewCriteria().Where("status = ?", "pending")
//	store.Updates(ctx, map[string]any{"status": "completed"}, criteria)
func (r *GormStore[M]) Updates(ctx context.Context, attributes any, criteria *Criteria) *gorm.DB {
	var model M
	tx := r.present(ctx, criteria).Model(&model).Updates(attributes)
	r.reset()
	return tx
}

// Save updates or inserts a record (upsert operation).
// If the model has a primary key, it updates; otherwise, it inserts.
//
// Example:
//
//	user.ID = 123
//	user.Name = "Updated Name"
//	store.Save(ctx, user)
func (r *GormStore[M]) Save(ctx context.Context, model M) *gorm.DB {
	tx := r.present(ctx, nil).Save(&model)
	r.reset()
	return tx
}

// FindByIDs retrieves multiple records by their primary key IDs.
// Returns an error if the IDs slice is empty.
//
// Example:
//
//	users, err := store.FindByIDs(ctx, []int64{1, 2, 3})
func (r *GormStore[M]) FindByIDs(ctx context.Context, ids []int64) ([]M, error) {
	var models []M
	if len(ids) < 1 {
		return nil, ErrEmptyID
	}
	err := r.present(ctx, nil).Find(&models, ids).Error
	r.reset()
	if err != nil {
		return nil, err
	}
	return models, nil
}

// FindByID retrieves a single record by its primary key ID.
// Returns an error if the record is not found.
//
// Example:
//
//	user, err := store.FindByID(ctx, 123)
func (r *GormStore[M]) FindByID(ctx context.Context, id any) (*M, error) {
	var model M
	err := r.present(ctx, nil).First(&model, id).Error
	r.reset()
	if err != nil {
		return nil, err
	}
	return &model, nil
}

// First retrieves the first record matching the given criteria.
// Returns an error if no matching record is found.
//
// Example:
//
//	criteria := storeit.NewCriteria().Where("status = ?", "active")
//	user, err := store.First(ctx, criteria)
func (r *GormStore[M]) First(ctx context.Context, criteria *Criteria) (*M, error) {
	var model M
	err := r.present(ctx, criteria).Take(&model).Error
	r.reset()
	if err != nil {
		return nil, err
	}
	return &model, nil
}

// Exists checks if any records match the given criteria.
// Returns true if at least one record exists, false otherwise.
//
// Example:
//
//	criteria := storeit.NewCriteria().Where("email = ?", "test@example.com")
//	exists, err := store.Exists(ctx, criteria)
func (r *GormStore[M]) Exists(ctx context.Context, criteria *Criteria) (bool, error) {
	count, err := r.Count(ctx, criteria)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Update updates a single column for records matching the criteria.
//
// Example:
//
//	criteria := storeit.NewCriteria().Where("id = ?", 123)
//	store.Update(ctx, "status", "active", criteria)
func (r *GormStore[M]) Update(ctx context.Context, column string, value any, criteria *Criteria) *gorm.DB {
	var model M
	tx := r.present(ctx, criteria).Model(&model).Update(column, value)
	r.reset()
	return tx
}

// UpdateById updates a single column for the record with the given ID.
//
// Example:
//
//	store.UpdateById(ctx, 123, "status", "active")
func (r *GormStore[M]) UpdateById(ctx context.Context, id any, column string, value any) *gorm.DB {
	var model M
	tx := r.present(ctx, nil).Model(&model).Where("id", id).Update(column, value)
	r.reset()
	return tx
}

// UpdatesById updates multiple columns for the record with the given ID.
//
// Example:
//
//	store.UpdatesById(ctx, 123, map[string]any{"status": "active", "updated_at": time.Now()})
func (r *GormStore[M]) UpdatesById(ctx context.Context, id any, updates any) *gorm.DB {
	var model M
	tx := r.present(ctx, nil).Model(&model).Where("id", id).Updates(updates)
	r.reset()
	return tx
}

// FindInBatches retrieves records in batches of the specified size.
// The callback function fc is called for each batch.
// This method uses the primary key for efficient batching.
//
// Example:
//
//	var allUsers []User
//	err := store.FindInBatches(ctx, &allUsers, 100, func(tx *gorm.DB, batch int) error {
//	    fmt.Printf("Processing batch %d\n", batch)
//	    return nil
//	}, criteria)
func (r *GormStore[M]) FindInBatches(ctx context.Context, models *[]M, batchSize int, fc func(tx *gorm.DB, batch int) error, criteria *Criteria) error {
	err := r.present(ctx, criteria).FindInBatches(models, batchSize, fc).Error
	r.reset()
	return err
}

// QueryInBatches retrieves records in batches without using the primary key.
// This is useful when the primary key is not sequential or for complex queries.
// Uses a streaming cursor approach for memory efficiency.
//
// Example:
//
//	var batchUsers []User
//	err := store.QueryInBatches(ctx, &batchUsers, 100, func(tx *gorm.DB, batch int) error {
//	    // Process batchUsers
//	    return nil
//	}, criteria)
func (r *GormStore[M]) QueryInBatches(ctx context.Context, models *[]M, batchSize int, fc func(tx *gorm.DB, batch int) error, criteria *Criteria) error {
	if models == nil {
		return ErrNilModels
	}
	if batchSize <= 0 {
		return ErrInvalidBatch
	}
	if fc == nil {
		return ErrNilCallback
	}
	// Ensure state is reset in all cases
	defer r.reset()
	var model M
	// Use Rows to get a streaming cursor
	db := r.present(ctx, criteria).Model(&model)
	rows, err := db.Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	currentBatch := 1
	// Preallocate memory to avoid frequent reallocations
	batchData := make([]M, 0, batchSize)

	for rows.Next() {
		var m M
		// Scan data into the model
		if err := db.ScanRows(rows, &m); err != nil {
			return err
		}
		batchData = append(batchData, m)

		// Trigger callback when batch size is reached
		if len(batchData) == batchSize {
			*models = batchData // Modify external pointer content
			if err := fc(db, currentBatch); err != nil {
				return err
			}
			// Reset current batch, reusing memory space
			batchData = make([]M, 0, batchSize)
			currentBatch++
		}
	}

	// Process the final batch if it has remaining data
	if len(batchData) > 0 {
		*models = batchData
		if err := fc(db, currentBatch); err != nil {
			return err
		}
	}

	return rows.Err()
}

// Count returns the number of records matching the given criteria.
// Order and limit clauses are automatically removed for accurate counting.
//
// Example:
//
//	criteria := storeit.NewCriteria().Where("status = ?", "active")
//	count, err := store.Count(ctx, criteria)
func (r *GormStore[M]) Count(ctx context.Context, criteria *Criteria) (i int64, err error) {
	var model M
	c := cloneCriteria(criteria)
	c.unsetOrder()
	c.unsetLimit()
	err = r.present(ctx, c).Model(&model).Count(&i).Error
	r.reset()
	return
}

// Sum calculates the sum of values in the specified column for records matching the criteria.
// Order and limit clauses are automatically removed for accurate aggregation.
//
// Example:
//
//	criteria := storeit.NewCriteria().Where("status = ?", "completed")
//	total, err := store.Sum(ctx, "amount", criteria)
func (r *GormStore[M]) Sum(ctx context.Context, column string, criteria *Criteria) (sum float64, err error) {
	var model M
	var result struct {
		Total float64
	}
	if column == "" {
		err = ErrInvalidColumn
		return
	}
	c := cloneCriteria(criteria)
	c.unsetOrder()
	c.unsetLimit()
	err = r.present(ctx, c).Model(&model).Select("SUM(" + QuoteReservedWord(column) + ") as total").Scan(&result).Error
	r.reset()
	if err != nil {
		return
	}
	return result.Total, nil
}

// Avg calculates the average of values in the specified column for records matching the criteria.
// Order and limit clauses are automatically removed for accurate aggregation.
//
// Example:
//
//	criteria := storeit.NewCriteria().Where("status = ?", "completed")
//	avg, err := store.Avg(ctx, "rating", criteria)
func (r *GormStore[M]) Avg(ctx context.Context, column string, criteria *Criteria) (avg float64, err error) {
	var model M
	if column == "" {
		err = ErrInvalidColumn
		return
	}
	var result struct {
		Avg float64
	}
	c := cloneCriteria(criteria)
	c.unsetOrder()
	c.unsetLimit()
	err = r.present(ctx, c).Model(&model).Select("AVG(" + QuoteReservedWord(column) + ") as avg").Scan(&result).Error
	r.reset()
	if err != nil {
		return
	}
	return result.Avg, nil
}

// Scan executes the query and scans results into the destination variable.
// Useful for custom result types or projections.
//
// Example:
//
//	var results []struct {
//	    Name  string
//	    Count int
//	}
//	err := store.Scan(ctx, criteria, &results)
func (r *GormStore[M]) Scan(ctx context.Context, criteria *Criteria, dst any) (err error) {
	var model M
	err = r.present(ctx, criteria).Model(&model).Scan(dst).Error
	r.reset()
	return err
}

// Find retrieves all records matching the given criteria.
// Returns an empty slice if no records are found.
//
// Example:
//
//	criteria := storeit.NewCriteria().Where("status = ?", "active").Limit(10)
//	users, err := store.Find(ctx, criteria)
func (r *GormStore[M]) Find(ctx context.Context, criteria *Criteria) ([]M, error) {
	var models []M

	err := r.present(ctx, criteria).Find(&models).Error
	r.reset()

	if err != nil {
		return nil, err
	}

	return models, nil
}

// Pluck retrieves a single column's values into the destination slice.
//
// Example:
//
//	var emails []string
//	err := store.Pluck(ctx, "email", &emails, criteria)
func (r *GormStore[M]) Pluck(ctx context.Context, column string, dest any, criteria *Criteria) error {
	var model M
	err := r.present(ctx, criteria).Model(&model).Pluck(column, dest).Error
	r.reset()

	return err
}

// All retrieves all records without any filtering criteria.
//
// Example:
//
//	users, err := store.All(ctx)
func (r *GormStore[M]) All(ctx context.Context) ([]M, error) {
	return r.Find(ctx, nil)
}

// Paginate retrieves a paginated list of records matching the given criteria.
// It executes count and query operations concurrently for better performance.
// Default page is 1 and default per_page is 50 if not specified in criteria.
//
// Example:
//
//	criteria := storeit.NewCriteria().Where("status = ?", "active").Page(2).PerPage(20)
//	pagination, err := store.Paginate(ctx, criteria)
func (r *GormStore[M]) Paginate(ctx context.Context, criteria *Criteria) (*Pagination[M], error) {
	if criteria == nil {
		criteria = NewCriteria()
	}
	if criteria.GetPage() < 1 {
		criteria.Page(1)
	}
	if criteria.GetPerPage() < 1 {
		criteria.PerPage(50)
	}
	var (
		eg    *errgroup.Group
		total int64
		items []M
	)
	eg, groupCtx := errgroup.WithContext(ctx)
	// Clone stores for concurrent use to avoid race conditions
	countStore := r.onceClone()
	findStore := r.onceClone()

	eg.Go(func() error {
		var err error
		total, err = countStore.Count(groupCtx, criteria)
		return err
	})
	eg.Go(func() error {
		var err error
		items, err = findStore.Find(groupCtx, criteria)
		return err
	})
	err := eg.Wait()
	if err != nil {
		return nil, err
	}
	var pagination = Pagination[M]{
		Total:   total,
		Page:    criteria.GetPage(),
		PerPage: criteria.GetPerPage(),
		Items:   items,
	}
	return &pagination, nil
}

// ScopeClosure adds a custom scope closure to the store.
// It returns a new cloned store instance with the scope added.
//
// Example:
//
//	store.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
//	    return tx.Where("created_at > ?", time.Now().AddDate(0, -1, 0))
//	}).Find(ctx, criteria)
func (r *GormStore[M]) ScopeClosure(closure gormClosure) *GormStore[M] {
	nr := r.onceClone()
	nr.scopeClosures = append(nr.scopeClosures, closure)
	return nr
}

// AddPreload adds a preload clause for eager loading associations.
// It returns a new cloned store instance with the preload added.
//
// Example:
//
//	store.AddPreload("Orders").AddPreload("Orders.Items").Find(ctx, criteria)
func (r *GormStore[M]) AddPreload(name string, args ...any) *GormStore[M] {
	nr := r.onceClone()
	nr.scopeClosures = append(nr.scopeClosures, func(tx *gorm.DB) *gorm.DB {
		return tx.Preload(name, args...)
	})

	return nr
}

// present builds and returns the final GORM DB instance with all
// configured options applied (columns, hidden fields, scopes, criteria).
// This method is thread-safe and creates local copies of state to avoid races.
func (r *GormStore[M]) present(ctx context.Context, criteria *Criteria) *gorm.DB {
	// Lock to read all fields, preventing race conditions with reset()
	r.mu.Lock()
	tx := r.tx
	hidden := r.hidden
	columns := r.columns
	scopeClosures := r.scopeClosures
	unscoped := r.unscoped
	r.mu.Unlock()

	var db *gorm.DB
	if tx != nil {
		db = tx.WithContext(ctx)
	} else {
		db = r.db.WithContext(ctx)
	}

	// Create local copy to avoid modifying the original object
	var localScopeClosures []gormClosure
	if len(scopeClosures) > 0 {
		localScopeClosures = append(localScopeClosures, scopeClosures...)
	}

	if len(hidden) > 0 {
		db = db.Omit(hidden...)
	}
	if len(columns) > 0 {
		db = db.Select(columns)
	}
	if unscoped {
		db = db.Unscoped()
	}
	if criteria != nil {
		if criteria.GetOffset() > 0 {
			db = db.Offset(criteria.GetOffset())
		}
		// Offset requires a limit
		if criteria.limit > 0 || criteria.GetOffset() > 0 {
			db = db.Limit(criteria.limit)
		}
		if criteria.group != "" {
			db = db.Group(criteria.group)
		}
		for _, item := range criteria.orders {
			db = db.Order(item)
		}
		// Use local copy instead of directly modifying r.scopeClosures
		if len(criteria.scopeClosures) > 0 {
			localScopeClosures = append(localScopeClosures, criteria.scopeClosures...)
		}
	}

	// Apply all scope closures
	if len(localScopeClosures) > 0 {
		for _, closure := range localScopeClosures {
			db = closure(db)
		}
	}
	return db
}

// onceClone creates a new store instance with copied state from the current store.
// This implements the immutable pattern - methods return new instances rather than
// modifying the original, preventing side effects.
func (r *GormStore[M]) onceClone() *GormStore[M] {
	r.mu.Lock()
	defer r.mu.Unlock()

	newStore := New[M](r.db)
	if len(r.scopeClosures) > 0 {
		newStore.scopeClosures = append(newStore.scopeClosures, r.scopeClosures...)
	}
	if len(r.hidden) > 0 {
		newStore.hidden = append(newStore.hidden, r.hidden...)
	}
	if len(r.columns) > 0 {
		newStore.columns = append(newStore.columns, r.columns...)
	}
	newStore.unscoped = r.unscoped
	newStore.tx = r.tx

	return newStore
}

// reset clears temporary query state after a database operation.
func (r *GormStore[M]) reset() *GormStore[M] {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.columns = nil
	r.hidden = nil
	r.scopeClosures = nil
	r.unscoped = false

	return r
}

// addColumns adds columns to select in queries.
// Returns a new cloned store instance with the columns added.
func (r *GormStore[M]) addColumns(columns []string) *GormStore[M] {
	if len(columns) == 0 {
		return r
	}
	nr := r.onceClone()
	nr.columns = append(nr.columns, columns...)

	return nr
}

// addHiddenColumns adds columns to omit from query results.
// Returns a new cloned store instance with the hidden columns added.
func (r *GormStore[M]) addHiddenColumns(columns []string) *GormStore[M] {
	if len(columns) == 0 {
		return r
	}
	nr := r.onceClone()
	nr.hidden = append(nr.hidden, columns...)

	return nr
}

func cloneCriteria(criteria *Criteria) *Criteria {
	c := &Criteria{}
	if criteria == nil {
		return c
	}
	c.limit = criteria.limit
	c.offset = criteria.offset
	c.group = criteria.group
	c.page = criteria.page
	if len(criteria.scopeClosures) > 0 {
		c.scopeClosures = append(c.scopeClosures, criteria.scopeClosures...)
	}
	if len(criteria.orders) > 0 {
		c.orders = append(c.orders, criteria.orders...)
	}
	return c
}
