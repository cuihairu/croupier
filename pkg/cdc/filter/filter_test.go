package filter

import (
	"context"
	"reflect"
	"testing"

	"github.com/cuihairu/croupier/pkg/cdc"
)

func TestFilterMatchDefaultDenyAndExcludeWins(t *testing.T) {
	event := cdc.ChangeEvent{SourceID: "main", SourceType: "mysql", Database: "game", Table: "users", Operation: cdc.OpUpdate}
	if (Filter{}).Match(event) {
		t.Fatal("empty include must deny")
	}
	f := Filter{
		Include: []Selector{{SourceType: "mysql", Database: "game", Table: "user*", Operations: []cdc.Operation{cdc.OpUpdate, cdc.OpInsert}}},
		Exclude: []Selector{{Table: "users", Operations: []cdc.Operation{cdc.OpUpdate}}},
	}
	if f.Match(event) {
		t.Fatal("exclude should override include")
	}
	event.Operation = cdc.OpInsert
	if !f.Match(event) {
		t.Fatal("expected included insert")
	}
	event.Database = "other"
	if f.Match(event) {
		t.Fatal("database selector should constrain match")
	}
}

func TestPushdownTablesOnlyForExactSafeIncludes(t *testing.T) {
	f := Filter{Include: []Selector{
		{SourceID: "main", Database: "game", Table: "users"},
		{SourceID: "main", Database: "game", Table: "sessions"},
	}}
	got := f.PushdownTables("main", "mysql")
	want := []string{"game.sessions", "game.users"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PushdownTables() = %#v, want %#v", got, want)
	}
	f.Include = append(f.Include, Selector{Database: "game", Table: "user*"})
	if got := f.PushdownTables("main", "mysql"); got != nil {
		t.Fatalf("broad selector should disable pushdown, got %#v", got)
	}
}

func TestEventsFiltersAndStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	in := make(chan cdc.ChangeEvent, 2)
	in <- cdc.ChangeEvent{Database: "db", Table: "wanted", Operation: cdc.OpInsert}
	in <- cdc.ChangeEvent{Database: "db", Table: "ignored", Operation: cdc.OpInsert}
	close(in)
	out := (Filter{Include: []Selector{{Database: "db", Table: "wanted"}}}).Events(ctx, in)
	if got := <-out; got.Table != "wanted" {
		t.Fatalf("unexpected event: %#v", got)
	}
	if _, ok := <-out; ok {
		t.Fatal("output should close after input")
	}
	cancel()
}
