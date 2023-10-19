package storeit

import (
	"context"
	"fmt"
	"sync"

	"github.com/jinzhu/copier"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

type gormClosure func(tx *gorm.DB) *gorm.DB

type Pagination[M any] struct {
	Total   int64 `json:"total"`
	PerPage int   `json:"per_page"`
	Page    int   `json:"page"`
	Items   []M   `json:"items"`
}

type preloadEntry struct {
	name string
	args any
}

type GormStore[M interface{}] struct {
	db            *gorm.DB
	preloads      []preloadEntry
	columns       []string
	hidden        []string
	scopeClosures []gormClosure
	cloned        bool
	mu            sync.Mutex
}

func New[M any](db *gorm.DB) *GormStore[M] {
	return &GormStore[M]{
		db:            db,
		columns:       make([]string, 0, 3),
		hidden:        make([]string, 0, 3),
		scopeClosures: make([]gormClosure, 0, 1),
		preloads:      make([]preloadEntry, 0, 1),
	}
}

func (r *GormStore[M]) Insert(ctx context.Context, model *M) *gorm.DB {
	tx := r.db.WithContext(ctx).Create(model)
	r.reset()
	return tx
}

func (r *GormStore[M]) Hidden(fields []string) *GormStore[M] {
	return r.addHiddenColumns(fields)
}

func (r *GormStore[M]) Columns(fields []string) *GormStore[M] {
	return r.addColumns(fields)
}

func (r *GormStore[M]) Create(ctx context.Context, model M) *gorm.DB {
	tx := r.present(ctx, nil).Create(&model)
	r.reset()
	return tx
}

func (r *GormStore[M]) Creates(ctx context.Context, models []M) *gorm.DB {
	tx := r.present(ctx, nil).Create(&models)
	r.reset()
	return tx
}

func (r *GormStore[M]) CreateInBatches(ctx context.Context, models []M, batchSize int) *gorm.DB {
	tx := r.present(ctx, nil).CreateInBatches(&models, batchSize)
	r.reset()
	return tx
}

func (r *GormStore[M]) Delete(ctx context.Context, model *M) *gorm.DB {
	tx := r.present(ctx, nil).Delete(model)
	r.reset()
	return tx
}

func (r *GormStore[M]) Deletes(ctx context.Context, criteria *Criteria) *gorm.DB {
	var model M
	tx := r.present(ctx, criteria).Delete(&model)
	r.reset()
	return tx
}

func (r *GormStore[M]) DeleteById(ctx context.Context, id any) *gorm.DB {
	var model M
	tx := r.present(ctx, nil).Delete(&model, &id)
	r.reset()

	return tx
}

func (r *GormStore[M]) Updates(ctx context.Context, attributes any, criteria *Criteria) *gorm.DB {
	var model M
	tx := r.present(ctx, criteria).Model(&model).Updates(attributes)
	r.reset()
	return tx
}

func (r *GormStore[M]) Save(ctx context.Context, model *M) *gorm.DB {
	tx := r.present(ctx, nil).Save(&model)
	return tx
}

func (r *GormStore[M]) Update(ctx context.Context, column string, value interface{}, criteria *Criteria) *gorm.DB {
	var model M
	tx := r.present(ctx, criteria).Model(&model).Update(column, value)
	r.reset()
	return tx
}

// FindByIDs find the result by IDs
func (r *GormStore[M]) FindByIDs(ctx context.Context, ids []int64) ([]M, error) {
	var models []M
	if len(ids) < 1 {
		return nil, fmt.Errorf("id is empty")
	}
	err := r.present(ctx, nil).Find(&models, ids).Error
	r.reset()
	if err != nil {
		return nil, err
	}
	return models, err
}

// FindByID find the result by ID
func (r *GormStore[M]) FindByID(ctx context.Context, id any) (*M, error) {
	var model M
	err := r.present(ctx, nil).First(&model, id).Error
	r.reset()
	if err != nil {
		return nil, err
	}
	return &model, err
}

// First Execute the query and get the first result.
func (r *GormStore[M]) First(ctx context.Context, criteria *Criteria) (*M, error) {
	var model M
	err := r.present(ctx, criteria).Take(&model).Error
	r.reset()
	if err != nil {
		return nil, err
	}

	return &model, err
}

// Count Retrieve the "count" result of the query.
func (r *GormStore[M]) Count(ctx context.Context, criteria *Criteria) (i int64, err error) {
	var c Criteria
	var model M
	err = copier.Copy(&c, criteria)
	if err != nil {
		return
	}
	c.unsetOrder()
	c.unsetLimit()
	err = r.present(ctx, &c).Model(&model).Count(&i).Error
	return
}

// Sum Retrieve the sum of the values of a given column.
func (r *GormStore[M]) Sum(ctx context.Context, column string, criteria *Criteria) (sum int64, err error) {
	var c Criteria
	var model M
	err = copier.Copy(&c, criteria)
	if err != nil {
		return
	}
	c.unsetOrder()
	c.unsetLimit()
	row := r.present(ctx, &c).Model(&model).Select("SUM(" + column + ")").Row()
	if row.Err() != nil {
		return
	}
	err = row.Scan(&sum)
	return
}

// Avg Retrieve the average of the values of a given column.
func (r *GormStore[M]) Avg(ctx context.Context, column string, criteria *Criteria) (avg int64, err error) {
	var c Criteria
	var model M
	err = copier.Copy(&c, criteria)
	if err != nil {
		return
	}
	c.unsetOrder()
	c.unsetLimit()
	row := r.present(ctx, &c).Model(&model).Select("AVG(" + column + ")").Row()
	if row.Err() != nil {
		return
	}
	err = row.Scan(&avg)
	return
}

func (r *GormStore[M]) Find(ctx context.Context, criteria *Criteria) ([]M, error) {
	var models []M

	err := r.present(ctx, criteria).Find(&models).Error
	r.reset()

	if err != nil {
		return nil, err
	}

	return models, nil
}

func (r *GormStore[M]) All(ctx context.Context) ([]M, error) {
	return r.Find(ctx, nil)
}

func (r *GormStore[M]) Paginate(ctx context.Context, criteria *Criteria) (*Pagination[M], error) {
	if criteria.GetPage() < 1 {
		criteria.Page(1)
	}
	if criteria.GetPerPage() < 1 {
		criteria.PerPage(50)
	}
	var (
		eg    errgroup.Group
		total int64
		items []M
	)
	eg.Go(func() error {
		var err error
		total, err = r.Count(ctx, criteria)
		return err
	})
	eg.Go(func() error {
		var err error
		items, err = r.Find(ctx, criteria)
		return err
	})
	if err := eg.Wait(); err != nil {
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

func (r *GormStore[M]) ScopeClosure(closure gormClosure) *GormStore[M] {
	nr := r.onceClone()
	if nr.scopeClosures == nil {
		nr.scopeClosures = make([]gormClosure, 2)
	}
	nr.scopeClosures = append(nr.scopeClosures, closure)
	return r
}

func (r *GormStore[M]) AddPreload(name string, args ...any) *GormStore[M] {
	nr := r.onceClone()
	if nr.preloads == nil {
		nr.preloads = make([]preloadEntry, 0, 2)
	}
	nr.preloads = append(nr.preloads, preloadEntry{
		name: name,
		args: args,
	})

	return nr
}

func (r *GormStore[M]) reset() *GormStore[M] {
	r.columns = nil
	r.hidden = nil
	r.preloads = nil

	return r
}

func (r *GormStore[M]) present(ctx context.Context, criteria *Criteria) *gorm.DB {
	db := r.db.WithContext(ctx)
	if r.preloads != nil {
		for _, p := range r.preloads {
			if p.name == "" {
				continue
			}
			if p.args == nil {
				db = db.Preload(p.name)
			} else {
				db = db.Preload(p.name, p.args)
			}
		}
	}
	if r.scopeClosures != nil {
		for _, closure := range r.scopeClosures {
			db = closure(db)
		}
	}
	if r.hidden != nil {
		db = db.Omit(r.hidden...)
	}
	if r.columns != nil && len(r.columns) > 0 {
		db = db.Select(r.columns)
	}
	if criteria != nil {
		for _, group := range criteria.groupOrConditions {
			if len(group) == 0 {
				continue
			} else if len(group) == 1 {
				db = db.Where(group[0].query, group[0].args...)
			} else {
				db1 := db.Where(group[0].query, group[0].args...)
				for i, spec := range group {
					if i > 1 {
						db1 = db1.Or(spec.query, spec.args...)
					}
				}
				db = db.Where(db1)
			}
		}
		for _, item := range criteria.whereConditions {
			db = db.Where(item.query, item.args...)
		}
		for _, item := range criteria.orConditions {
			db = db.Or(item.query, item.args)
		}
		for _, item := range criteria.notConditions {
			db = db.Not(item.query, item.args)
		}
		for _, item := range criteria.havingConditions {
			db = db.Having(item.query, item.args)
		}
		for _, item := range criteria.joinConditions {
			db = db.Joins(item.query, item.args)
		}
		for _, item := range criteria.orders {
			db = db.Order(item)
		}
		if criteria.GetOffset() > 0 {
			db = db.Offset(criteria.GetOffset())
		}
		// 有 offset 一定要有 limit
		if criteria.limit > 0 || criteria.GetOffset() > 0 {
			db = db.Limit(criteria.limit)
		}
	}
	return db
}

func (r *GormStore[M]) addColumns(columns []string) *GormStore[M] {
	nr := r.onceClone()
	nr.columns = append(nr.columns, columns...)

	return nr
}

func (r *GormStore[M]) addHiddenColumns(columns []string) *GormStore[M] {
	nr := r.onceClone()
	nr.hidden = append(nr.hidden, columns...)

	return nr
}

func (r *GormStore[M]) onceClone() *GormStore[M] {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cloned {
		return r
	}
	newStore := New[M](r.db)
	if r.scopeClosures != nil && len(r.scopeClosures) > 0 {
		newStore.scopeClosures = r.scopeClosures
	}
	if r.hidden != nil && len(r.hidden) > 0 {
		newStore.hidden = r.hidden
	}
	if r.preloads != nil && len(r.preloads) > 0 {
		newStore.preloads = r.preloads
	}
	if r.columns != nil && len(r.columns) > 0 {
		newStore.columns = r.columns
	}
	newStore.cloned = true
	return newStore
}
