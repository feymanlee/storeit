// Package storeit provides a generic repository pattern wrapper around GORM.
// This file contains utilities for handling MySQL reserved words.
package storeit

import (
	"fmt"
	"strings"
	"sync"
)

// mysqlReservedWords is a list of MySQL reserved keywords that need to be
// escaped with backticks when used as identifiers (table names, column names, etc.).
// Using these words unescaped in SQL queries can cause syntax errors.
//
// Source: MySQL 8.0 Reserved Words
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

// Variables for lazy initialization of the reserved words map.
// Using sync.Once ensures thread-safe, one-time initialization.
var (
	reservedWordsMap     map[string]bool // Map for O(1) lookup of reserved words
	reservedWordsMapOnce sync.Once       // Ensures map is initialized only once
)

// initReservedWordsMap initializes the reserved words map from the slice.
// This is called lazily on first use via sync.Once.
func initReservedWordsMap() {
	reservedWordsMapOnce.Do(func() {
		reservedWordsMap = make(map[string]bool, len(mysqlReservedWords))
		for _, word := range mysqlReservedWords {
			reservedWordsMap[strings.ToUpper(word)] = true
		}
	})
}

// IsMySQLReservedWord checks if a word is a MySQL reserved keyword.
// The check is case-insensitive.
//
// Example:
//
//	if storeit.IsMySQLReservedWord("order") {
//	    // Handle reserved word
//	}
func IsMySQLReservedWord(word string) bool {
	initReservedWordsMap()
	return reservedWordsMap[strings.ToUpper(word)]
}

// QuoteReservedWord escapes a word with backticks if it is a MySQL reserved word.
// It also handles:
//   - Empty strings (returned as-is)
//   - Already quoted words (returned as-is)
//   - Table-qualified column names (e.g., "users.order" becomes "users.`order`")
//
// Example:
//
//	QuoteReservedWord("name")        // Returns: "name"
//	QuoteReservedWord("order")       // Returns: "`order`"
//	QuoteReservedWord("users.order") // Returns: "users.`order`"
//	QuoteReservedWord("`order`")     // Returns: "`order`" (already quoted)
func QuoteReservedWord(word string) string {
	// Handle empty string
	if word == "" {
		return word
	}

	// Return as-is if already wrapped in backticks
	if strings.HasPrefix(word, "`") && strings.HasSuffix(word, "`") {
		return word
	}

	// Handle table.column syntax
	if strings.Contains(word, ".") {
		parts := strings.Split(word, ".")
		for i, part := range parts {
			if part != "" && IsMySQLReservedWord(part) {
				parts[i] = fmt.Sprintf("`%s`", part)
			}
		}
		return strings.Join(parts, ".")
	}

	// Handle regular field names
	if IsMySQLReservedWord(word) {
		return fmt.Sprintf("`%s`", word)
	}
	return word
}
