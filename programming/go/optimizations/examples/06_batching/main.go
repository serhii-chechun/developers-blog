/*
	06_batching contrasts one round trip per item with a single batched call.

$ go run main.go
*/
package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Item struct {
	ID    int
	Value string
}

// store mimics a database with a fixed per-call latency.
type store struct{}

func (store) Exec(_ context.Context, _ string, _ ...any) error {
	time.Sleep(500 * time.Microsecond) // network + query overhead
	return nil
}

// InsertOne does one round trip per item - the N+1 pattern.
func InsertOne(ctx context.Context, s store, items []Item) error {
	for _, it := range items {
		if err := s.Exec(ctx, "INSERT INTO items VALUES (?, ?)", it.ID, it.Value); err != nil {
			return err
		}
	}
	return nil
}

// InsertBatch does one round trip for all items.
func InsertBatch(ctx context.Context, s store, items []Item) error {
	args := make([]any, 0, len(items)*2)
	for _, it := range items {
		args = append(args, it.ID, it.Value)
	}
	return s.Exec(ctx, "INSERT INTO items (id, value) VALUES "+placeholders(len(items)), args...)
}

// placeholders builds "(?, ?), (?, ?), ..." for the batch INSERT.
func placeholders(n int) string {
	var out strings.Builder
	for i := range n {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString("(?, ?)")
	}
	return out.String()
}

func main() {
	ctx := context.Background()
	items := make([]Item, 100)
	for i := range items {
		items[i] = Item{ID: i, Value: fmt.Sprintf("v%d", i)}
	}

	start := time.Now()
	_ = InsertOne(ctx, store{}, items)
	fmt.Printf("one-by-one: %v\n", time.Since(start))

	start = time.Now()
	_ = InsertBatch(ctx, store{}, items)
	fmt.Printf("batched:    %v\n", time.Since(start))
}
