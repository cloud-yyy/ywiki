package api

import (
	"encoding/json"
	"testing"
)

func TestFlexIDAcceptsStringAndNumber(t *testing.T) {
	var ids []FlexID
	if err := json.Unmarshal([]byte(`["a1", 7, null]`), &ids); err != nil {
		t.Fatalf("unmarshal returned error: %v", err)
	}

	if ids[0] != "a1" || ids[1] != "7" || ids[2] != "" {
		t.Errorf("ids = %q, want a1, 7, and empty", ids)
	}
}

func TestGridCellSendsNumericRowIDAsNumber(t *testing.T) {
	tests := []struct {
		cell GridCell
		want string
	}{
		{GridCell{RowID: "12", ColumnSlug: "a", Value: "x"}, `{"row_id":12,"column_slug":"a","value":"x"}`},
		{GridCell{RowID: "r-1", ColumnSlug: "a"}, `{"row_id":"r-1","column_slug":"a","value":null}`},
	}

	for _, tt := range tests {
		got, err := json.Marshal(tt.cell)
		if err != nil {
			t.Fatalf("marshal returned error: %v", err)
		}
		if string(got) != tt.want {
			t.Errorf("marshal = %s, want %s", got, tt.want)
		}
	}
}
