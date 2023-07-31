package storeit

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Name             string  `gorm:"size:255"`
	Age              int64   `gorm:"age"`
	Gender           string  `gorm:"gender"`
	Emails           []Email // One-To-Many (拥有多个 - Email表的UserID作外键)
	Value            int
	BillingAddress   Address // One-To-One (属于 - 本表的BillingAddressID作外键)
	BillingAddressID int64
}

type Email struct {
	gorm.Model
	UserID     int    `gorm:"index"`                          // 外键 (属于), tag `index`是为该列创建索引
	Email      string `gorm:"type:varchar(100);unique_index"` // `type`设置sql类型, `unique_index` 为该列设置唯一索引
	Subscribed bool
}

type Address struct {
	gorm.Model
	Address1 string         `gorm:"not null;unique"` // 设置字段为非空并唯一
	Address2 string         `gorm:"type:varchar(100);unique"`
	Post     sql.NullString `gorm:"not null"`
}

func setupTestDB() *gorm.DB {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	err = db.AutoMigrate(&User{}, &Email{}, &Address{})
	if err != nil {
		panic(err)
	}
	return db
}

func TestInsert(t *testing.T) {
	db := setupTestDB()
	defer clearTestData(db)
	store := New[User](db)
	model := &User{Name: "Test", Value: 123}

	tx := store.Insert(context.Background(), model)
	assert.NoError(t, tx.Error)

	var result User
	err := db.First(&result, model.ID).Error
	assert.NoError(t, err)
	assert.Equal(t, model.Name, result.Name)
	assert.Equal(t, model.Value, result.Value)
}

func TestDelete(t *testing.T) {
	db := setupTestDB()
	defer clearTestData(db)
	store := New[User](db)
	model := &User{Name: "Test", Value: 123}

	tx := store.Insert(context.Background(), model)
	assert.NoError(t, tx.Error)

	tx = store.Delete(context.Background(), model)
	assert.NoError(t, tx.Error)

	var result User
	err := db.First(&result, model.ID).Error
	assert.Error(t, err)
}

func TestFindByID(t *testing.T) {
	db := setupTestDB()
	defer clearTestData(db)
	store := New[User](db)
	model := &User{Name: "Test", Value: 123}

	tx := store.Insert(context.Background(), model)
	assert.NoError(t, tx.Error)

	result, err := store.FindByID(context.Background(), model.ID)
	assert.NoError(t, err)
	assert.Equal(t, model.Name, result.Name)
	assert.Equal(t, model.Value, result.Value)
	result, err = store.FindByID(context.Background(), 100)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestFindByIDs(t *testing.T) {
	db := setupTestDB()
	defer clearTestData(db)
	store := New[User](db)
	model1 := &User{Name: "Test 1", Value: 1}
	model2 := &User{Name: "Test 2", Value: 2}

	tx1 := store.Insert(context.Background(), model1)
	assert.NoError(t, tx1.Error)
	tx2 := store.Insert(context.Background(), model2)
	assert.NoError(t, tx2.Error)

	result, err := store.FindByIDs(context.Background(), []int64{int64(model1.ID), int64(model2.ID)})
	assert.NoError(t, err)
	assert.Equal(t, len(result), 2)
	result, err = store.FindByIDs(context.Background(), []int64{100, 300})
	assert.Error(t, err)
	assert.Empty(t, result)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestUpdate(t *testing.T) {
	db := setupTestDB()
	defer clearTestData(db)
	store := New[User](db)
	model := &User{Name: "Test", Value: 123}

	tx := store.Insert(context.Background(), model)
	assert.NoError(t, tx.Error)
	where := NewCriteria().Where("id=?", model.ID)
	model.Name = "Updated Test"
	model.Value = 456

	tx = store.Update(context.Background(), "name", "Updated Test", where)
	assert.NoError(t, tx.Error)
	tx = store.Update(context.Background(), "value", 456, where)
	assert.NoError(t, tx.Error)

	result, err := store.FindByID(context.Background(), model.ID)
	assert.NoError(t, err)
	assert.Equal(t, model.Name, result.Name)
	assert.Equal(t, model.Value, result.Value)
}

func TestAll(t *testing.T) {
	db := setupTestDB()
	defer clearTestData(db)
	store := New[User](db)

	model1 := &User{Name: "Test 1", Value: 1}
	model2 := &User{Name: "Test 2", Value: 2}

	store.Insert(context.Background(), model1)
	store.Insert(context.Background(), model2)

	results, err := store.All(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 2, len(results))
}

func TestFind(t *testing.T) {
	db := setupTestDB()
	defer clearTestData(db)
	store := New[User](db)

	model1 := &User{Name: "Test 1", Value: 1}
	model2 := &User{Name: "Test 2", Value: 2}

	store.Insert(context.Background(), model1)
	store.Insert(context.Background(), model2)

	criteria := NewCriteria().Where("value = ?", 1)
	results, err := store.Find(context.Background(), criteria)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, model1.Name, (results)[0].Name)
	assert.Equal(t, model1.Value, (results)[0].Value)
}

func TestCount(t *testing.T) {
	db := setupTestDB()
	defer clearTestData(db)
	store := New[User](db)

	model1 := &User{Name: "Test 1", Value: 1}
	model2 := &User{Name: "Test 2", Value: 2}

	store.Insert(context.Background(), model1)
	store.Insert(context.Background(), model2)

	criteria := NewCriteria().Where("value = ?", 1)
	count, err := store.Count(context.Background(), criteria)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestPaginate(t *testing.T) {
	db := setupTestDB()
	defer clearTestData(db)
	store := New[User](db)

	criteria := NewCriteria().Page(1).PerPage(2).Order("nalue", false)
	pagination, err := store.Paginate(context.Background(), criteria)
	assert.Error(t, err)
	assert.Nil(t, pagination)
	model1 := &User{Name: "Test 1", Value: 1}
	model2 := &User{Name: "Test 2", Value: 2}
	model3 := &User{Name: "Test 3", Value: 3}

	store.Insert(context.Background(), model1)
	store.Insert(context.Background(), model2)
	store.Insert(context.Background(), model3)

	criteria = NewCriteria().Page(1).PerPage(2).Order("value", false)
	pagination, err = store.Paginate(context.Background(), criteria)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), pagination.Total)
	assert.Equal(t, 2, pagination.PerPage)
	assert.Equal(t, 1, pagination.Page)
	assert.Equal(t, 2, len(pagination.Items))
}

func clearTestData(db *gorm.DB) {
	db.Where("1 = 1").Delete(&User{})
	db.Where("1 = 1").Delete(&Email{})
	db.Where("1 = 1").Delete(&Address{})
}

func TestGormStore_Hidden(t *testing.T) {
	db := setupTestDB()
	defer clearTestData(db)
	users := getTestUsers()
	store := New[User](db)
	store.Creates(context.Background(), users)
	first, err := store.Hidden([]string{"name", "updated_at"}).First(context.Background(), nil)
	assert.NoError(t, err)
	assert.Empty(t, first.Name)
	assert.Empty(t, first.UpdatedAt)
}

func TestGormStore_Creates(t *testing.T) {
	db := setupTestDB()
	defer clearTestData(db)
	users := getTestUsers()
	store := New[User](db)
	tx := store.Creates(context.Background(), users)
	assert.NoError(t, tx.Error)
	assert.Equal(t, tx.RowsAffected, int64(len(users)))
	assert.Equal(t, time.Now().Year(), users[0].CreatedAt.Year())
	assert.Equal(t, time.Now().Year(), users[0].UpdatedAt.Year())
	assert.True(t, users[0].ID != 0)
}

func TestGormStore_CreateInBatches(t *testing.T) {
	db := setupTestDB()
	defer clearTestData(db)
	users := getTestUsers()
	store := New[User](db)
	tx := store.CreateInBatches(context.Background(), users, 2)
	assert.NoError(t, tx.Error)
	assert.Equal(t, tx.RowsAffected, int64(len(users)))
	assert.Equal(t, time.Now().Year(), users[0].CreatedAt.Year())
	assert.Equal(t, time.Now().Year(), users[0].UpdatedAt.Year())
	assert.True(t, users[0].ID != 0)
}

func getTestUsers() []User {
	return []User{
		{
			Name:   "John Doe",
			Age:    30,
			Gender: "male",
			Value:  1000,
		},
		{
			Name:   "Jane Doe",
			Age:    28,
			Gender: "female",
			Value:  2000,
		},
		{
			Name:   "Alice",
			Age:    25,
			Gender: "female",
			Value:  1500,
		},
		{
			Name:   "Bob",
			Age:    32,
			Gender: "male",
			Value:  1200,
		},
		{
			Name:   "Charlie",
			Age:    22,
			Gender: "male",
			Value:  900,
		},
	}
}

func getEmails() []Email {
	return []Email{
		{UserID: 1, Email: "user1@example.com", Subscribed: true},
		{UserID: 2, Email: "user2@example.com", Subscribed: false},
		{UserID: 3, Email: "user3@example.com", Subscribed: true},
		{UserID: 4, Email: "user4@example.com", Subscribed: false},
		{UserID: 5, Email: "user5@example.com", Subscribed: true},
	}
}

func TestGormStore_Columns(t *testing.T) {
	db := setupTestDB()
	defer clearTestData(db)
	users := getTestUsers()
	store := New[User](db)
	store.Creates(context.Background(), users)
	first, err := store.Columns([]string{"name", "updated_at"}).First(context.Background(), nil)
	assert.NoError(t, err)
	assert.Empty(t, first.CreatedAt)
	assert.Empty(t, first.Value)
}

func TestGormStore_Deletes(t *testing.T) {
	db := setupTestDB()
	defer clearTestData(db)
	emails := getEmails()
	store := New[Email](db)
	store.Creates(context.Background(), emails)
	c := NewCriteria().Where("email = ?", "user5@example.com")
	tx := store.Deletes(context.Background(), c)
	assert.NoError(t, tx.Error)
	assert.Equal(t, int64(1), tx.RowsAffected)
}

func TestGormStore_DeleteById(t *testing.T) {
	db := setupTestDB()
	defer clearTestData(db)
	users := getTestUsers()
	store := New[User](db)
	store.Creates(context.Background(), users)
	first, err := store.First(context.Background(), nil)
	assert.NoError(t, err)
	tx := store.DeleteById(context.Background(), first.ID)
	assert.NoError(t, tx.Error)
	assert.Equal(t, tx.RowsAffected, int64(1))
	res, err := store.FindByID(context.Background(), first.ID)
	assert.Nil(t, res)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestGormStore_Updates(t *testing.T) {
	db := setupTestDB()
	defer clearTestData(db)
	users := getTestUsers()
	store := New[User](db)
	store.Creates(context.Background(), users)
	first, err := store.First(context.Background(), nil)
	assert.NoError(t, err)
	c := NewCriteria().Where("id=?", first.ID)
	tx := store.Updates(context.Background(), map[string]any{"name": "nameupdated"}, c)
	assert.NoError(t, tx.Error)
	assert.Equal(t, tx.RowsAffected, int64(1))
	res, err := store.FindByID(context.Background(), first.ID)
	assert.NoError(t, err)
	assert.Equal(t, res.Name, "nameupdated")
}

func TestGormStore_Save(t *testing.T) {
	db := setupTestDB()
	defer clearTestData(db)
	store := New[User](db)
	user := User{
		Name:   "Charlie",
		Age:    22,
		Gender: "male",
		Value:  900,
	}
	store.Save(context.Background(), &user)
}
