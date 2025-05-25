package storeit

import (
	"fmt"
	"strings"
	"sync"
)

var mysqlReservedWords = []string{
	"ADD", "ALL", "ALTER", "ANALYZE", "AND", "AS", "ASC", "ASENSITIVE",
	"BEFORE", "BETWEEN", "BIGINT", "BLOB", "BOTH", "BY", "CALL", "CASCADE",
	"CASE", "CHANGE", "CHAR", "CHARACTER", "CHECK", "COLLATE", "COLUMN",
	"CONDITION", "CONSTRAINT", "CONTINUE", "CONVERT", "CREATE", "CROSS",
	"CURRENT_DATE", "CURRENT_TIME", "CURRENT_TIMESTAMP", "CURRENT_USER",
	"CURSOR", "DATABASE", "DATABASES", "DAY_HOUR", "DAY_MICROSECOND",
	"DAY_MINUTE", "DAY_SECOND", "DEC", "DECIMAL", "DECLARE", "DEFAULT",
	"DELAYED", "DELETE", "DESC", "DESCRIBE", "DETERMINISTIC", "DISTINCT",
	"DISTINCTROW", "DIV", "DOUBLE", "DROP", "DUAL", "EACH", "ELSE", "ELSEIF",
	"ENCLOSED", "ESCAPED", "EXISTS", "EXIT", "EXPLAIN", "FALSE", "FETCH",
	"FLOAT", "FLOAT4", "FLOAT8", "FOR", "FORCE", "FOREIGN", "FROM", "FULLTEXT",
	"GRANT", "GROUP", "HAVING", "HIGH_PRIORITY", "HOUR_MICROSECOND",
	"HOUR_MINUTE", "HOUR_SECOND", "IF", "IGNORE", "IN", "INDEX", "INFILE",
	"INNER", "INOUT", "INSENSITIVE", "INSERT", "INT", "INT1", "INT2", "INT3",
	"INT4", "INT8", "INTEGER", "INTERVAL", "INTO", "IS", "ITERATE", "JOIN",
	"KEY", "KEYS", "KILL", "LEADING", "LEAVE", "LEFT", "LIKE", "LIMIT", "LINES",
	"LOAD", "LOCALTIME", "LOCALTIMESTAMP", "LOCK", "LONG", "LONGBLOB",
	"LONGTEXT", "LOOP", "LOW_PRIORITY", "MASTER_SSL_VERIFY_SERVER_CERT",
	"MATCH", "MAXVALUE", "MEDIUMBLOB", "MEDIUMINT", "MEDIUMTEXT", "MIDDLEINT",
	"MINUTE_MICROSECOND", "MINUTE_SECOND", "MOD", "MODIFIES", "NATURAL",
	"NOT", "NO_WRITE_TO_BINLOG", "NULL", "NUMERIC", "ON", "OPTIMIZE", "OPTION",
	"OPTIONALLY", "OR", "ORDER", "OUT", "OUTER", "OUTFILE", "PRECISION",
	"PRIMARY", "PROCEDURE", "PURGE", "RANGE", "READ", "READS", "READ_WRITE",
	"REAL", "REFERENCES", "REGEXP", "RELEASE", "RENAME", "REPEAT",
	"REPLACE", "REQUIRE", "RESTRICT", "RETURN", "REVOKE", "RIGHT", "RLIKE",
	"SCHEMA", "SCHEMAS", "SECOND_MICROSECOND", "SELECT", "SENSITIVE", "SEPARATOR",
	"SET", "SHOW", "SMALLINT", "SPATIAL", "SPECIFIC", "SQL", "SQLEXCEPTION",
	"SQLSTATE", "SQLWARNING", "SQL_BIG_RESULT", "SQL_CALC_FOUND_ROWS",
	"SQL_SMALL_RESULT", "SSL", "STARTING", "STRAIGHT_JOIN", "TABLE", "TERMINATED",
	"THEN", "TINYBLOB", "TINYINT", "TINYTEXT", "TO", "TRAILING", "TRIGGER",
	"TRUE", "UNDO", "UNION", "UNIQUE", "UNLOCK", "UNSIGNED", "UPDATE", "USAGE",
	"USE", "USING", "UTC_DATE", "UTC_TIME", "UTC_TIMESTAMP", "VALUES", "VARBINARY",
	"VARCHAR", "VARCHARACTER", "VARYING", "WHEN", "WHERE", "WHILE", "WITH",
	"WRITE", "XOR", "YEAR_MONTH", "ZEROFILL", "RANK", "OFFSET",
}

// 使用 map 存储保留字，提高查找效率
var (
	reservedWordsMap     map[string]bool
	reservedWordsMapOnce sync.Once
)

// 初始化保留字 map
func initReservedWordsMap() {
	reservedWordsMapOnce.Do(func() {
		reservedWordsMap = make(map[string]bool, len(mysqlReservedWords))
		for _, word := range mysqlReservedWords {
			reservedWordsMap[strings.ToUpper(word)] = true
		}
	})
}

// IsMySQLReservedWord 检查一个词是否是 MySQL 保留字
func IsMySQLReservedWord(word string) bool {
	initReservedWordsMap()
	return reservedWordsMap[strings.ToUpper(word)]
}

// QuoteReservedWord 如果是保留字，则用反引号包裹
func QuoteReservedWord(word string) string {
	// 处理空字符串
	if word == "" {
		return word
	}

	// 如果已经被引号包裹，则直接返回
	if strings.HasPrefix(word, "`") && strings.HasSuffix(word, "`") {
		return word
	}

	// 处理表名.列名的情况
	if strings.Contains(word, ".") {
		parts := strings.Split(word, ".")
		for i, part := range parts {
			if part != "" && IsMySQLReservedWord(part) {
				parts[i] = fmt.Sprintf("`%s`", part)
			}
		}
		return strings.Join(parts, ".")
	}

	// 处理普通字段名
	if IsMySQLReservedWord(word) {
		return fmt.Sprintf("`%s`", word)
	}
	return word
}
