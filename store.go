package storeit

import (
	"context"

	"github.com/jinzhu/copier"
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
	db            *gorm.DB
	preloads      map[string][]any
	columns       []string
	hidden        []string
	scopeClosures []gormClosure
}

func New[M any](db *gorm.DB) *GormStore[M] {
	return &GormStore[M]{
		db:            db,
		columns:       make([]string, 0, 3),
		hidden:        make([]string, 0, 3),
		scopeClosures: make([]gormClosure, 0, 1),
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

func (r *GormStore[M]) FindByID(ctx context.Context, id any) (*M, error) {
	var model M
	err := r.present(ctx, nil).First(&model, id).Error
	r.reset()
	if err != nil {
		return nil, err
	}
	return &model, err
}

func (r *GormStore[M]) First(ctx context.Context, criteria *Criteria) (*M, error) {
	var model M
	err := r.present(ctx, criteria).Take(&model).Error
	r.reset()
	if err != nil {
		return nil, err
	}

	return &model, err
}

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
	total, err := r.Count(ctx, criteria)
	if err != nil {
		return nil, err
	}
	items, err := r.Find(ctx, criteria)
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
	if r.scopeClosures == nil {
		r.scopeClosures = make([]gormClosure, 2)
	}
	r.scopeClosures = append(r.scopeClosures, closure)
	return r
}

func (r *GormStore[M]) AddPreload(preload string, args ...any) *GormStore[M] {
	if r.preloads == nil {
		r.preloads = make(map[string][]any, 5)
	}
	r.preloads[preload] = args

	return r
}

func (r *GormStore[M]) reset() *GormStore[M] {
	r.columns = nil
	r.hidden = nil
	r.preloads = nil

	return r
}

func (r *GormStore[M]) present(ctx context.Context, criteria *Criteria) *gorm.DB {
	db := r.db.WithContext(ctx)
	for p, args := range r.preloads {
		if args == nil {
			db = db.Preload(p)
		} else {
			db = db.Preload(p, args...)
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
		if offset := criteria.GetOffset(); offset > 0 {
			db = db.Offset(offset)
		}
		if criteria.limit > 0 {
			db = db.Limit(criteria.limit)
		}
	}
	return db
}

func (r *GormStore[M]) addColumns(columns []string) *GormStore[M] {
	r.columns = append(r.columns, columns...)

	return r
}

func (r *GormStore[M]) addHiddenColumns(columns []string) *GormStore[M] {
	r.hidden = append(r.hidden, columns...)

	return r
}
