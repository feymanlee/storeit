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

type GormStore[M interface{}] struct {
	tx            *gorm.DB
	db            *gorm.DB
	columns       []string
	hidden        []string
	scopeClosures []gormClosure
	mu            sync.Mutex
	unscoped      bool
}

func New[M any](db *gorm.DB) *GormStore[M] {
	return &GormStore[M]{
		db: db,
	}
}

func (r *GormStore[M]) SetTx(tx *gorm.DB) *GormStore[M] {
	if tx == nil {
		return r
	}
	nr := r.onceClone()
	nr.tx = tx
	return nr
}

func (r *GormStore[M]) Insert(ctx context.Context, model *M) *gorm.DB {
	var tx *gorm.DB
	if r.tx != nil {
		tx = r.tx.WithContext(ctx).Create(model)
	} else {
		tx = r.db.WithContext(ctx).Create(model)
	}
	r.reset()
	return tx
}

func (r *GormStore[M]) Unscoped() *GormStore[M] {
	nr := r.onceClone()
	nr.unscoped = true
	return nr
}

func (r *GormStore[M]) WithTrashed(with bool) *GormStore[M] {
	nr := r.onceClone()
	nr.unscoped = with
	return nr
}

func (r *GormStore[M]) Hidden(fields []string) *GormStore[M] {
	return r.addHiddenColumns(fields)
}

func (r *GormStore[M]) Emit(fields []string) *GormStore[M] {
	nr := r.onceClone()
	return nr.Hidden(fields)
}

func (r *GormStore[M]) Columns(fields []string) *GormStore[M] {
	return r.addColumns(fields)
}

func (r *GormStore[M]) Create(ctx context.Context, model *M) *gorm.DB {
	tx := r.present(ctx, nil).Create(model)
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

func (r *GormStore[M]) Save(ctx context.Context, model M) *gorm.DB {
	tx := r.present(ctx, nil).Save(&model)
	r.reset() // 添加这一行，确保状态被重置
	return tx
}

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
	return models, nil // 修改为返回 nil 而不是 err
}

func (r *GormStore[M]) FindByID(ctx context.Context, id any) (*M, error) {
	var model M
	err := r.present(ctx, nil).First(&model, id).Error
	r.reset()
	if err != nil {
		return nil, err
	}
	return &model, nil // 修改为返回 nil 而不是 err
}

func (r *GormStore[M]) First(ctx context.Context, criteria *Criteria) (*M, error) {
	var model M
	err := r.present(ctx, criteria).Take(&model).Error
	r.reset()
	if err != nil {
		return nil, err
	}
	return &model, nil // 修改为返回 nil 而不是 err
}

func (r *GormStore[M]) Exists(ctx context.Context, criteria *Criteria) (bool, error) {
	count, err := r.Count(ctx, criteria)
	if err != nil {
		return false, err
	}
	// 移除这里的 r.reset() 调用，因为 Count 方法已经调用了
	return count > 0, nil
}

func (r *GormStore[M]) Update(ctx context.Context, column string, value interface{}, criteria *Criteria) *gorm.DB {
	var model M
	tx := r.present(ctx, criteria).Model(&model).Update(column, value)
	r.reset()
	return tx
}

func (r *GormStore[M]) UpdateById(ctx context.Context, id any, column string, value interface{}) *gorm.DB {
	var model M
	tx := r.present(ctx, nil).Model(&model).Where("id = ?", id).Update(column, value)
	r.reset()
	return tx
}

func (r *GormStore[M]) UpdatesById(ctx context.Context, id any, updates interface{}) *gorm.DB {
	var model M
	tx := r.present(ctx, nil).Model(&model).Where("id = ?", id).Updates(updates)
	r.reset()
	return tx
}

// FindInBatches finds all records in batches of batchSize
func (r *GormStore[M]) FindInBatches(ctx context.Context, models *[]M, batchSize int, fc func(tx *gorm.DB, batch int) error, criteria *Criteria) error {
	err := r.present(ctx, criteria).FindInBatches(models, batchSize, fc).Error
	r.reset()
	return err
}

// Count Retrieve the "count" result of the query.
func (r *GormStore[M]) Count(ctx context.Context, criteria *Criteria) (i int64, err error) {
	var c Criteria
	var model M
	if criteria != nil {
		err = copier.Copy(&c, criteria)
		if err != nil {
			return
		}
	}
	c.unsetOrder()
	c.unsetLimit()
	err = r.present(ctx, &c).Model(&model).Count(&i).Error
	r.reset()
	return
}

// Sum Retrieve the sum of the values of a given column.
func (r *GormStore[M]) Sum(ctx context.Context, column string, criteria *Criteria) (sum float64, err error) {
	var c Criteria
	var model M
	var result struct {
		Total float64
	}
	if criteria != nil {
		err = copier.Copy(&c, criteria)
		if err != nil {
			return
		}
	}
	c.unsetOrder()
	c.unsetLimit()
	err = r.present(ctx, &c).Model(&model).Select("SUM(" + column + ") as total").Scan(&result).Error
	r.reset()
	if err != nil {
		return
	}
	return result.Total, nil
}

// Avg Retrieve the average of the values of a given column.
func (r *GormStore[M]) Avg(ctx context.Context, column string, criteria *Criteria) (avg float64, err error) {
	var c Criteria
	var model M
	if criteria != nil {
		err = copier.Copy(&c, criteria)
		if err != nil {
			return
		}
	}
	var result struct {
		Avg float64
	}
	c.unsetOrder()
	c.unsetLimit()
	err = r.present(ctx, &c).Model(&model).Select("AVG(" + column + ") as avg").Scan(&result).Error
	r.reset()
	if err != nil {
		return
	}
	return result.Avg, nil
}

func (r *GormStore[M]) Scan(ctx context.Context, criteria *Criteria, dst any) (err error) {
	var model M
	err = r.present(ctx, criteria).Model(&model).Scan(dst).Error
	r.reset()
	return err
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

func (r *GormStore[M]) Pluck(ctx context.Context, column string, dest any, criteria *Criteria) error {
	var model M
	err := r.present(ctx, criteria).Model(&model).Pluck(column, dest).Error
	r.reset()

	return err
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

func (r *GormStore[M]) ScopeClosure(closure gormClosure) *GormStore[M] {
	nr := r.onceClone()
	nr.scopeClosures = append(nr.scopeClosures, closure)
	return nr
}

func (r *GormStore[M]) AddPreload(name string, args ...any) *GormStore[M] {
	nr := r.onceClone()
	nr.scopeClosures = append(nr.scopeClosures, func(tx *gorm.DB) *gorm.DB {
		return tx.Preload(name, args...)
	})

	return nr
}

func (r *GormStore[M]) present(ctx context.Context, criteria *Criteria) *gorm.DB {
	var db *gorm.DB
	if r.tx != nil {
		db = r.tx.WithContext(ctx)
	} else {
		db = r.db.WithContext(ctx)
	}

	// 创建本地副本，避免修改原始对象
	var localScopeClosures []gormClosure
	if len(r.scopeClosures) > 0 {
		localScopeClosures = append(localScopeClosures, r.scopeClosures...)
	}

	if len(r.hidden) > 0 {
		db = db.Omit(r.hidden...)
	}
	if len(r.columns) > 0 {
		db = db.Select(r.columns)
	}
	if r.unscoped {
		db = db.Unscoped()
	}
	if criteria != nil {
		if criteria.GetOffset() > 0 {
			db = db.Offset(criteria.GetOffset())
		}
		// 有 offset 一定要有 limit
		if criteria.limit > 0 || criteria.GetOffset() > 0 {
			db = db.Limit(criteria.limit)
		}
		if criteria.group != "" {
			db = db.Group(criteria.group)
		}
		for _, item := range criteria.orders {
			db = db.Order(item)
		}
		// 使用本地副本而不是直接修改 r.scopeClosures
		if len(criteria.scopeClosures) > 0 {
			localScopeClosures = append(localScopeClosures, criteria.scopeClosures...)
		}
	}

	// 使用本地副本
	if len(localScopeClosures) > 0 {
		for _, closure := range localScopeClosures {
			db = closure(db)
		}
	}
	return db
}

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

func (r *GormStore[M]) reset() *GormStore[M] {
	r.columns = nil
	r.hidden = nil
	r.scopeClosures = nil
	r.unscoped = false
	r.tx = nil

	return r
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
