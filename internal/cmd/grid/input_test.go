package grid

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/cloud-yyy/ywiki/internal/api"
)

func testGrid() *api.Grid {
	return &api.Grid{Structure: api.GridStructure{Columns: []api.GridColumn{
		{Slug: "name", Type: api.ColumnString},
		{Slug: "qty", Type: api.ColumnNumber},
		{Slug: "ok", Type: api.ColumnCheckbox},
		{Slug: "tags", Type: api.ColumnSelect, Multiple: true},
		{Slug: "status", Type: api.ColumnSelect},
		{Slug: "owner", Type: api.ColumnStaff},
	}}}
}

func TestParseRows(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []map[string]any
		wantErr bool
	}{
		{
			name:  "json array",
			input: `[{"name":"a","qty":1},{"name":"b"}]`,
			want:  []map[string]any{{"name": "a", "qty": float64(1)}, {"name": "b"}},
		},
		{
			name:  "object stream",
			input: "{\"name\":\"a\"}\n{\"name\":\"b\"}\n",
			want:  []map[string]any{{"name": "a"}, {"name": "b"}},
		},
		{
			name:  "single object",
			input: `{"name":"a"}`,
			want:  []map[string]any{{"name": "a"}},
		},
		{
			name:  "csv converts by column type",
			input: "name,qty,ok,tags\na,2.5,false,\"x, y\"\n",
			want: []map[string]any{{
				"name": "a", "qty": json.Number("2.5"), "ok": false, "tags": []string{"x", "y"},
			}},
		},
		{
			name:  "csv sends a single select as a one-item list",
			input: "name,status\na,open\n",
			want:  []map[string]any{{"name": "a", "status": []string{"open"}}},
		},
		{name: "csv rejects plain text for staff", input: "owner\nme\n", wantErr: true},
		{
			name:  "csv takes json for staff",
			input: "name,owner\na,\"[{\"\"login\"\":\"\"me\"\"}]\"\n",
			want:  []map[string]any{{"name": "a", "owner": []any{map[string]any{"login": "me"}}}},
		},
		{
			name:  "row id is ignored",
			input: `[{"_id":"4","name":"a"}]`,
			want:  []map[string]any{{"name": "a"}},
		},
		{name: "empty", input: "  \n", wantErr: true},
		{name: "unknown column", input: `[{"nope":1}]`, wantErr: true},
		{name: "unknown csv column", input: "nope\n1\n", wantErr: true},
		{name: "csv bad number", input: "qty\nmany\n", wantErr: true},
		{name: "csv header only", input: "name,qty\n", wantErr: true},
		{name: "json array of non-objects", input: `[1,2]`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRows(tt.input, testGrid())
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseRows error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRows = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseCells(t *testing.T) {
	cells, err := parseCells(
		`[{"row_id":3,"column":"qty","value":7},{"row_id":"4","column_slug":"name","value":null}]`,
		testGrid())
	if err != nil {
		t.Fatalf("parseCells returned error: %v", err)
	}

	want := []api.GridCell{
		{RowID: "3", ColumnSlug: "qty", Value: float64(7)},
		{RowID: "4", ColumnSlug: "name", Value: nil},
	}
	if !reflect.DeepEqual(cells, want) {
		t.Errorf("parseCells = %#v, want %#v", cells, want)
	}

	for _, bad := range []string{`[]`, `not json`, `[{"column":"qty"}]`, `[{"row_id":1,"column":"nope"}]`} {
		if _, err := parseCells(bad, testGrid()); err == nil {
			t.Errorf("parseCells(%q) succeeded, want an error", bad)
		}
	}
}

func TestParseColumnSpec(t *testing.T) {
	tests := []struct {
		spec    string
		want    api.GridColumn
		wantErr bool
	}{
		{spec: "name:string", want: api.GridColumn{Slug: "name", Title: "name", Type: "string"}},
		{spec: "price:number:Unit price", want: api.GridColumn{Slug: "price", Title: "Unit price", Type: "number"}},
		{spec: "when:date:Due: soon", want: api.GridColumn{Slug: "when", Title: "Due: soon", Type: "date"}},
		{spec: "name", wantErr: true},
		{spec: ":string", wantErr: true},
		{spec: "name:text", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			got, err := parseColumnSpec(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseColumnSpec error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseColumnSpec = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseSort(t *testing.T) {
	got := parseSort("name, -qty,")
	want := []api.GridSort{{"name": "asc"}, {"qty": "desc"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseSort = %v, want %v", got, want)
	}
}

func TestCellText(t *testing.T) {
	tests := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"x", "x"},
		{float64(10), "10"},
		{2.5, "2.5"},
		{true, "true"},
		{[]any{"a", float64(1)}, "a, 1"},
		{map[string]any{"login": "me", "id": float64(3)}, "me"},
		{map[string]any{"x": float64(1)}, `{"x":1}`},
	}

	for _, tt := range tests {
		if got := cellText(tt.in); got != tt.want {
			t.Errorf("cellText(%#v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
