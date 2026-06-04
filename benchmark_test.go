package storeit

import "testing"

func BenchmarkExtractCriteria(b *testing.B) {
	req := testCriteriaStruct{
		Name:     "alice",
		Age:      18,
		Email:    "alice@example.com",
		Status:   "active",
		Page:     2,
		PerPage:  20,
		SortBy:   "name-,age+",
		Offset:   5,
		Limit:    20,
		Keywords: "engineer",
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		criteria, err := ExtractCriteria(req)
		if err != nil {
			b.Fatal(err)
		}
		if criteria.GetLimit() != 20 {
			b.Fatalf("unexpected limit: %d", criteria.GetLimit())
		}
	}
}

func BenchmarkQuoteQualifiedReservedWord(b *testing.B) {
	fields := []string{"id", "name", "order", "users.order", "created_at", "rank"}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		field := fields[i%len(fields)]
		if QuoteReservedWord(field) == "" {
			b.Fatal("quoted field is empty")
		}
	}
}

func BenchmarkCriteriaWhereChain(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		criteria := NewCriteria().
			Where("status = ?", "active").
			WhereGt("age", 18).
			WhereContains("name", "ali").
			OrderDesc("created_at").
			Page(2).
			PerPage(20)
		if criteria.GetOffset() != 20 {
			b.Fatalf("unexpected offset: %d", criteria.GetOffset())
		}
	}
}
