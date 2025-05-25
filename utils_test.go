package storeit

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsMySQLReservedWord(t *testing.T) {
	tests := []struct {
		name     string
		word     string
		expected bool
	}{
		{
			name:     "保留字 SELECT",
			word:     "SELECT",
			expected: true,
		},
		{
			name:     "保留字 select (小写)",
			word:     "select",
			expected: true,
		},
		{
			name:     "保留字 ORDER",
			word:     "ORDER",
			expected: true,
		},
		{
			name:     "保留字 group (小写)",
			word:     "group",
			expected: true,
		},
		{
			name:     "非保留字",
			word:     "username",
			expected: false,
		},
		{
			name:     "空字符串",
			word:     "",
			expected: false,
		},
		{
			name:     "混合大小写保留字",
			word:     "OrDeR",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsMySQLReservedWord(tt.word)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQuoteReservedWord(t *testing.T) {
	tests := []struct {
		name     string
		word     string
		expected string
	}{
		{
			name:     "保留字",
			word:     "order",
			expected: "`order`",
		},
		{
			name:     "非保留字",
			word:     "username",
			expected: "username",
		},
		{
			name:     "空字符串",
			word:     "",
			expected: "",
		},
		{
			name:     "已被引号包裹的保留字",
			word:     "`order`",
			expected: "`order`",
		},
		{
			name:     "表名.列名 (两者都是保留字)",
			word:     "order.group",
			expected: "`order`.`group`",
		},
		{
			name:     "表名.列名 (表名是保留字)",
			word:     "order.username",
			expected: "`order`.username",
		},
		{
			name:     "表名.列名 (列名是保留字)",
			word:     "users.order",
			expected: "users.`order`",
		},
		{
			name:     "表名.列名 (都不是保留字)",
			word:     "users.username",
			expected: "users.username",
		},
		{
			name:     "多级表名.列名",
			word:     "schema.table.column",
			expected: "`schema`.`table`.`column`",
		},
		{
			name:     "多级表名.列名 (部分是保留字)",
			word:     "schema.order.key",
			expected: "`schema`.`order`.`key`",
		},
		{
			name:     "带有点但不是表名.列名格式的字符串",
			word:     ".",
			expected: ".",
		},
		{
			name:     "以点开始的字符串",
			word:     ".column",
			expected: ".`column`",
		},
		{
			name:     "以点结束的字符串",
			word:     "table.",
			expected: "`table`.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := QuoteReservedWord(tt.word)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInitReservedWordsMap(t *testing.T) {
	// 确保 map 初始化正确
	initReservedWordsMap()
	assert.NotNil(t, reservedWordsMap)
	assert.Greater(t, len(reservedWordsMap), 0)

	// 验证所有保留字都在 map 中
	for _, word := range mysqlReservedWords {
		assert.True(t, reservedWordsMap[word])
	}

	// 验证 map 大小与保留字数组大小一致
	assert.Equal(t, len(mysqlReservedWords), len(reservedWordsMap))
}

func TestReservedWordsMapConcurrency(t *testing.T) {
	// 测试并发初始化
	done := make(chan bool)

	// 并发调用 10 次初始化函数
	for i := 0; i < 10; i++ {
		go func() {
			initReservedWordsMap()
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证 map 只被初始化一次
	assert.NotNil(t, reservedWordsMap)
	assert.Equal(t, len(mysqlReservedWords), len(reservedWordsMap))
}

func TestMySQLReservedWordsContent(t *testing.T) {
	// 验证一些常见的 MySQL 保留字确实在列表中
	commonReservedWords := []string{
		"SELECT", "INSERT", "UPDATE", "DELETE", "FROM",
		"WHERE", "JOIN", "GROUP", "ORDER", "HAVING",
		"LIMIT", "OFFSET", "TABLE", "INDEX", "PRIMARY",
		"KEY", "FOREIGN", "CONSTRAINT", "CASCADE", "REFERENCES",
	}

	for _, word := range commonReservedWords {
		if !IsMySQLReservedWord(word) {
			fmt.Println(word)
		}
		assert.True(t, IsMySQLReservedWord(word), "常见保留字 %s 应该在保留字列表中", word)
	}
}

func BenchmarkIsMySQLReservedWord(b *testing.B) {
	// 初始化 map
	initReservedWordsMap()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsMySQLReservedWord("SELECT")
		IsMySQLReservedWord("username")
	}
}

func BenchmarkQuoteReservedWord(b *testing.B) {
	for i := 0; i < b.N; i++ {
		QuoteReservedWord("order")
		QuoteReservedWord("username")
		QuoteReservedWord("table.column")
		QuoteReservedWord("order.group")
	}
}
