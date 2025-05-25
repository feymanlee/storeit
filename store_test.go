package storeit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type TestModel struct {
	ID        uint   `gorm:"primarykey,column:id"`
	Name      string `gorm:"size:255,column:name"`
	Age       int    `gorm:"size:3,column:age"`
	Score     int    `gorm:"size:3,column:score"`
	Email     string `gorm:"size:255,column:email"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
}

type Address struct {
	ID     uint   `gorm:"primarykey,column:id"`
	UserId uint   `gorm:"column:user_id"`
	Street string `gorm:"column:street"`
}

type UserWithAddress struct {
	TestModel
	Addresses []Address `gorm:"foreignKey:UserId"`
}

type Department struct {
	ID   uint   `gorm:"primarykey"`
	Name string `gorm:"size:255"`
}

type Employee struct {
	TestModel
	DepartmentID uint       `gorm:"column:department_id"`
	Department   Department `gorm:"foreignKey:DepartmentID"`
}

func (Employee) TableName() string {
	return "employees"
}

func (UserWithAddress) TableName() string {
	return "user_with_address"
}

// TableName 指定表名
func (TestModel) TableName() string {
	return "test_models"
}

func setupTestDB(t *testing.T) *gorm.DB {
	// 使用共享内存数据库，避免多连接导致的表丢失
	dbName := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())

	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{
		SkipDefaultTransaction:                   true,
		Logger:                                   logger.Default.LogMode(logger.Info),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	assert.NoError(t, err)

	// 确保表结构正确
	err = db.AutoMigrate(&TestModel{})
	assert.NoError(t, err)

	// 添加清理函数
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
	})

	return db
}

func TestGormStore_Basic(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// Test Create
	model := &TestModel{
		Name:  "Test User",
		Age:   25,
		Email: "test@example.com",
	}
	err := store.Create(ctx, model).Error
	assert.NoError(t, err)
	assert.NotZero(t, model.ID)

	// Test FindByID
	found, err := store.FindByID(ctx, model.ID)
	assert.NoError(t, err)
	assert.Equal(t, model.Name, found.Name)

	// Test Update
	err = store.UpdateById(ctx, model.ID, "age", 26).Error
	assert.NoError(t, err)

	// Test First
	criteria := NewCriteria().Where("age", 26)
	updated, err := store.First(ctx, criteria)
	assert.NoError(t, err)
	assert.Equal(t, 26, updated.Age)

	// Test Delete
	err = store.DeleteById(ctx, model.ID).Error
	assert.NoError(t, err)

	// Test Soft Delete
	_, err = store.FindByID(ctx, model.ID)
	assert.Error(t, err)

	// Test Unscoped Find
	found, err = store.Unscoped().FindByID(ctx, model.ID)
	assert.NoError(t, err)
	assert.NotNil(t, found.DeletedAt)
}

func TestGormStore_BatchOperations(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// Test Creates
	models := []TestModel{
		{Name: "User 1", Age: 20},
		{Name: "User 2", Age: 25},
		{Name: "User 3", Age: 30},
	}
	err := store.Creates(ctx, models).Error
	assert.NoError(t, err)

	// Test FindInBatches
	var results []TestModel
	batchSize := 2
	err = store.FindInBatches(ctx, &results, batchSize, func(tx *gorm.DB, batch int) error {
		assert.LessOrEqual(t, len(results), batchSize)
		return nil
	}, nil)
	assert.NoError(t, err)

	// Test Paginate
	criteria := NewCriteria().Page(1).PerPage(2)
	pagination, err := store.Paginate(ctx, criteria)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), pagination.Total)
	assert.Equal(t, 2, len(pagination.Items))
}

func TestGormStore_Aggregations(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// Prepare data
	models := []TestModel{
		{Name: "User 1", Age: 20},
		{Name: "User 2", Age: 25},
		{Name: "User 3", Age: 30},
	}
	err := store.Creates(ctx, models).Error
	assert.NoError(t, err)

	// Test Count
	count, err := store.Count(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// Test Exists
	exists, err := store.Exists(ctx, NewCriteria().WhereGt("age", 25))
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test Sum
	sum, err := store.Sum(ctx, "age", nil)
	assert.NoError(t, err)
	assert.Equal(t, float64(75), sum)

	// Test Avg
	avg, err := store.Avg(ctx, "age", nil)
	assert.NoError(t, err)
	assert.Equal(t, float64(25), avg)
}

func TestGormStore_Columns(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	model := &TestModel{
		Name:  "Test User",
		Age:   25,
		Email: "test@example.com",
	}
	err := store.Create(ctx, model).Error
	assert.NoError(t, err)

	// Test Select Columns
	store = store.Columns([]string{"name", "age"})
	found, err := store.FindByID(ctx, model.ID)
	assert.NoError(t, err)
	assert.NotEmpty(t, found.Name)
	assert.NotZero(t, found.Age)
	assert.Empty(t, found.Email)

	// Test Hidden Columns
	store = New[TestModel](db).Hidden([]string{"email"})
	found, err = store.FindByID(ctx, model.ID)
	assert.NoError(t, err)
	assert.NotEmpty(t, found.Name)
	assert.NotZero(t, found.Age)
	assert.Empty(t, found.Email)
}

func TestGormStore_SetTx(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)

	// 测试设置事务
	tx := db.Begin()
	txStore := store.SetTx(tx)
	assert.NotNil(t, txStore)

	// 测试重复设置事务
	txStore2 := txStore.SetTx(tx)
	assert.Equal(t, txStore, txStore2)

	tx.Rollback()
}

func TestGormStore_Transaction(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// 测试事务回滚
	tx := db.Begin()
	txStore := store.SetTx(tx)

	model := &TestModel{
		Name: "Transaction Test",
		Age:  30,
	}

	err := txStore.Create(ctx, model).Error
	assert.NoError(t, err)

	tx.Rollback()

	// 验证数据已回滚
	count, err := store.Count(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestGormStore_Clone(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)

	// 测试克隆
	store1 := store.Columns([]string{"name"})
	store2 := store.Hidden([]string{"email"})

	assert.NotEqual(t, store1.columns, store2.columns)
	assert.NotEqual(t, store1.hidden, store2.hidden)
}

func TestGormStore_FindByIDs(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// 准备测试数据
	models := []TestModel{
		{Name: "User 1", Age: 20},
		{Name: "User 2", Age: 25},
		{Name: "User 3", Age: 30},
	}
	err := store.Creates(ctx, models).Error
	assert.NoError(t, err)

	// 测试空ID列表
	_, err = store.FindByIDs(ctx, []int64{})
	assert.Error(t, err)

	// 测试正常查询
	var ids []int64
	for _, m := range models {
		ids = append(ids, int64(m.ID))
	}
	found, err := store.FindByIDs(ctx, ids)
	assert.NoError(t, err)
	assert.Equal(t, len(models), len(found))
}

func TestGormStore_Pluck(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// 准备测试数据
	models := []TestModel{
		{Name: "User 1", Age: 20},
		{Name: "User 2", Age: 25},
		{Name: "User 3", Age: 30},
	}
	err := store.Creates(ctx, models).Error
	assert.NoError(t, err)

	// 测试 Pluck
	var names []string
	err = store.Pluck(ctx, "name", &names, nil)
	assert.NoError(t, err)
	assert.Equal(t, len(models), len(names))
	assert.Contains(t, names, "User 1")
}

func TestGormStore_Scan(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// 准备测试数据
	models := []TestModel{
		{Name: "User 1", Age: 20},
		{Name: "User 2", Age: 25},
	}
	err := store.Creates(ctx, models).Error
	assert.NoError(t, err)

	// 测试 Scan
	type Result struct {
		Name string
		Age  int
	}
	var results []Result
	err = store.Scan(ctx, nil, &results)
	assert.NoError(t, err)
	assert.Equal(t, len(models), len(results))
}

func TestGormStore_WithTrashed(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// 创建并软删除记录
	model := &TestModel{
		Name: "Deleted User",
		Age:  25,
	}
	err := store.Create(ctx, model).Error
	assert.NoError(t, err)

	err = store.DeleteById(ctx, model.ID).Error
	assert.NoError(t, err)

	// 测试 WithTrashed
	found, err := store.WithTrashed(true).FindByID(ctx, model.ID)
	assert.NoError(t, err)
	assert.NotNil(t, found)
	assert.NotNil(t, found.DeletedAt)
}

func TestGormStore_CreateInBatches(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// 准备大量测试数据
	var models []TestModel
	for i := 0; i < 100; i++ {
		models = append(models, TestModel{
			Name: fmt.Sprintf("User %d", i),
			Age:  20 + i%30,
		})
	}

	// 测试批量创建
	err := store.CreateInBatches(ctx, models, 10).Error
	assert.NoError(t, err)

	// 验证数据
	count, err := store.Count(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(100), count)
}

func TestGormStore_Save(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// 测试创建
	model := TestModel{
		Name: "Save Test",
		Age:  25,
	}
	err := store.Create(ctx, &model).Error
	assert.NoError(t, err)

	// 测试更新
	model.Age = 26
	err = store.Save(ctx, model).Error
	assert.NoError(t, err)

	// 验证更新
	found, err := store.FindByID(ctx, model.ID)
	assert.NoError(t, err)
	assert.Equal(t, 26, found.Age)
}

func TestGormStore_Updates(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// 准备测试数据
	models := []TestModel{
		{Name: "User 1", Age: 20, Email: "user1@test.com"},
		{Name: "User 2", Age: 25, Email: "user2@test.com"},
	}
	err := store.Creates(ctx, models).Error
	assert.NoError(t, err)

	// 测试批量更新
	criteria := NewCriteria().Where("age < ?", 25)
	updates := map[string]interface{}{
		"age":   30,
		"email": "updated@test.com",
	}
	err = store.Updates(ctx, updates, criteria).Error
	assert.NoError(t, err)

	// 验证更新结果
	updated, err := store.First(ctx, NewCriteria().Where("age = ?", 30))
	assert.NoError(t, err)
	assert.Equal(t, "updated@test.com", updated.Email)
}

func TestGormStore_All(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// 准备测试数据
	models := []TestModel{
		{Name: "User 1", Age: 20},
		{Name: "User 2", Age: 25},
	}
	err := store.Creates(ctx, models).Error
	assert.NoError(t, err)

	// 测试获取所有记录
	all, err := store.All(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(models), len(all))
}

func TestGormStore_Find(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// 准备测试数据
	models := []TestModel{
		{Name: "User 1", Age: 20},
		{Name: "User 2", Age: 25},
		{Name: "User 3", Age: 30},
	}
	err := store.Creates(ctx, models).Error
	assert.NoError(t, err)

	// 测试复杂查询条件
	criteria := NewCriteria().
		Where("age >= ?", 20).
		Where("age <= ?", 25).
		OrderDesc("age").
		Limit(2)

	found, err := store.Find(ctx, criteria)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(found))
	assert.Equal(t, 25, found[0].Age)
}

func TestGormStore_ScopeClosure(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// 准备测试数据
	models := []TestModel{
		{Name: "User 1", Age: 20},
		{Name: "User 2", Age: 25},
	}
	err := store.Creates(ctx, models).Error
	assert.NoError(t, err)

	// 测试自定义作用域
	store = store.ScopeClosure(func(tx *gorm.DB) *gorm.DB {
		return tx.Where("age > ?", 20)
	})

	found, err := store.Find(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(found))
	assert.Equal(t, 25, found[0].Age)
}

func TestGormStore_AddPreload(t *testing.T) {

	db := setupTestDB(t)
	err := db.AutoMigrate(&Address{})
	assert.NoError(t, err)
	err = db.AutoMigrate(&UserWithAddress{})
	assert.NoError(t, err)
	store := New[UserWithAddress](db)
	ctx := context.Background()

	// 创建测试数据
	user := &UserWithAddress{
		TestModel: TestModel{
			Name: "User with Address",
			Age:  25,
		},
		Addresses: []Address{
			{Street: "Street 1"},
			{Street: "Street 2"},
		},
	}
	err = store.Create(ctx, user).Error
	assert.NoError(t, err)

	// 测试预加载
	store = store.AddPreload("Addresses")
	found, err := store.FindByID(ctx, user.ID)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(found.Addresses))
}

func TestGormStore_Emit(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	model := &TestModel{
		Name:  "Test User",
		Age:   25,
		Email: "test@example.com",
	}
	err := store.Create(ctx, model).Error
	assert.NoError(t, err)

	// 测试 Emit (等同于 Hidden)
	store = store.Emit([]string{"email", "age"})
	found, err := store.FindByID(ctx, model.ID)
	assert.NoError(t, err)
	assert.NotEmpty(t, found.Name)
	assert.Zero(t, found.Age)
	assert.Empty(t, found.Email)
}

func TestGormStore_Reset(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)

	// 添加一些设置
	store = store.
		Columns([]string{"name"}).
		Hidden([]string{"email"}).
		ScopeClosure(func(tx *gorm.DB) *gorm.DB {
			return tx.Where("age > ?", 20)
		})

	// 测试重置
	store = store.reset()
	assert.Empty(t, store.columns)
	assert.Empty(t, store.hidden)
	assert.Empty(t, store.scopeClosures)
	assert.False(t, store.unscoped)
	assert.Nil(t, store.tx)
}

func TestGormStore_Present(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// 测试各种条件组合
	criteria := NewCriteria().
		Page(2).
		PerPage(10).
		Order("name", false).
		Group("age").
		WhereGt("age", 20)

	tx := store.present(ctx, criteria)
	assert.NotNil(t, tx)
}

func TestGormStore_ErrorHandling(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// 测试查询不存在的记录
	_, err := store.FindByID(ctx, 999)
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)

	// 测试无效的列名
	err = store.Update(ctx, "invalid_column", "value", nil).Error
	assert.Error(t, err)

	// 测试无效的聚合字段
	_, err = store.Sum(ctx, "invalid_column", nil)
	assert.Error(t, err)

	// 测试无效的排序字段
	criteria := NewCriteria().Order("invalid_column", false)
	_, err = store.Find(ctx, criteria)
	assert.Error(t, err)
}

func TestGormStore_Deletes(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// 准备测试数据
	models := []TestModel{
		{Name: "User 1", Age: 20},
		{Name: "User 2", Age: 25},
		{Name: "User 3", Age: 30},
	}
	err := store.Creates(ctx, models).Error
	assert.NoError(t, err)

	// 测试批量删除
	criteria := NewCriteria().WhereLt("age", 26)
	err = store.Deletes(ctx, criteria).Error
	assert.NoError(t, err)

	// 验证删除结果
	count, err := store.Count(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// 测试 WithTrashed 查询被删除的记录
	count, err = store.WithTrashed(true).Count(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestGormStore_Transaction_Commit(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// 开始事务
	tx := db.Begin()
	txStore := store.SetTx(tx)

	// 在事务中执行操作
	model := &TestModel{
		Name: "Transaction Commit Test",
		Age:  30,
	}
	err := txStore.Create(ctx, model).Error
	assert.NoError(t, err)

	// 提交事务
	tx.Commit()

	// 验证数据已提交
	found, err := store.FindByID(ctx, model.ID)
	assert.NoError(t, err)
	assert.Equal(t, model.Name, found.Name)
}

func TestGormStore_ComplexQueries(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// 准备测试数据
	models := []TestModel{
		{Name: "User 1", Age: 20, Email: "user1@test.com"},
		{Name: "User 2", Age: 25, Email: "user2@test.com"},
		{Name: "User 3", Age: 30, Email: "user3@test.com"},
	}
	err := store.Creates(ctx, models).Error
	assert.NoError(t, err)

	// 测试组合条件查询
	criteria := NewCriteria().
		WhereGte("age", 20).
		WhereLte("age", 30).
		OrderDesc("age").
		Group("age").
		Having("COUNT(*) > ?", 0)

	found, err := store.Find(ctx, criteria)
	assert.NoError(t, err)
	assert.NotEmpty(t, found)

	// 测试 OR 条件
	criteria = NewCriteria().
		Where("age", 20).
		OrWhere("age", 30)

	found, err = store.Find(ctx, criteria)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(found))
}

func TestGormStore_Joins(t *testing.T) {

	db := setupTestDB(t)

	// 清理之前可能存在的表
	err := db.Migrator().DropTable(&Employee{}, &Department{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&Department{}, &Employee{})
	assert.NoError(t, err)

	store := New[Employee](db)
	ctx := context.Background()

	// 创建测试数据
	dept := Department{Name: "IT"}
	err = db.Create(&dept).Error
	assert.NoError(t, err)

	employee := &Employee{
		TestModel: TestModel{
			Name: "Join Test",
			Age:  25,
		},
		DepartmentID: dept.ID,
	}
	err = store.Create(ctx, employee).Error
	assert.NoError(t, err)

	// 测试 Joins
	found, err := store.First(ctx, NewCriteria().Joins("LEFT JOIN departments ON departments.id = employees.department_id").Where("departments.name = ?", "IT"))
	assert.NoError(t, err)
	assert.Equal(t, employee.ID, found.ID)
	// 测试结束后清理表
	err = db.Migrator().DropTable(&Employee{}, &Department{})
	assert.NoError(t, err)
}

func TestGormStore_Pagination_EdgeCases(t *testing.T) {
	db := setupTestDB(t)
	store := New[TestModel](db)
	ctx := context.Background()

	// 准备测试数据
	var models []TestModel
	for i := 0; i < 15; i++ {
		models = append(models, TestModel{
			Name: fmt.Sprintf("User %d", i),
			Age:  20 + i,
		})
	}
	err := store.Creates(ctx, models).Error
	assert.NoError(t, err)

	// 测试第一页
	pagination, err := store.Paginate(ctx, NewCriteria().Page(1).PerPage(5))
	assert.NoError(t, err)
	assert.Equal(t, 5, len(pagination.Items))
	assert.Equal(t, int64(15), pagination.Total)

	// 测试最后一页
	pagination, err = store.Paginate(ctx, NewCriteria().Page(3).PerPage(5))
	assert.NoError(t, err)
	assert.Equal(t, 5, len(pagination.Items))

	// 测试超出范围的页码
	pagination, err = store.Paginate(ctx, NewCriteria().Page(4).PerPage(5))
	assert.NoError(t, err)
	assert.Empty(t, pagination.Items)

	// 测试每页大小为0
	pagination, err = store.Paginate(ctx, NewCriteria().Page(1).PerPage(0))
	assert.NoError(t, err)
	assert.Equal(t, int64(15), pagination.Total)
	assert.Equal(t, 15, len(pagination.Items))
}
