package grid_test

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/cloud-yyy/ywiki/internal/cmd/grid"
	wikierrors "github.com/cloud-yyy/ywiki/internal/errors"
	"github.com/cloud-yyy/ywiki/internal/testutil"
)

const gridJSON = `{
	"id": "g1", "title": "Stock", "revision": "r1", "created_at": "2026-01-02T03:04:05Z",
	"page": {"id": 12345, "slug": "users/me/notes", "title": "Notes"},
	"structure": {"columns": [
		{"slug": "name", "title": "Name", "type": "string"},
		{"slug": "qty", "title": "Qty", "type": "number"},
		{"slug": "ok", "title": "OK", "type": "checkbox"},
		{"slug": "tags", "title": "Tags", "type": "select", "multiple": true}
	]},
	"rows": [
		{"id": "1", "row": ["bolt", 10, true, ["a", "b"]]},
		{"id": 2, "row": ["nut", 20]}
	]
}`

// gridServer serves the table on GET /grids/g1 and acknowledges every write
// with a new revision. Handlers can override a path through routes.
func gridServer(t *testing.T, routes map[string]string) *testutil.RequestLog {
	t.Helper()

	log := &testutil.RequestLog{}
	testutil.StubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		log.Record(r)
		w.Header().Set("Content-Type", "application/json")

		key := r.Method + " " + r.URL.Path
		if body, ok := routes[key]; ok {
			_, _ = w.Write([]byte(body))
			return
		}

		switch {
		case key == "GET /grids/g1":
			_, _ = w.Write([]byte(gridJSON))
		case r.Method == http.MethodGet && r.URL.Path == "/pages":
			_, _ = w.Write([]byte(`{"id":12345,"slug":"users/me/notes"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/grids/g1":
			w.WriteHeader(http.StatusNoContent)
		default:
			_, _ = w.Write([]byte(`{"revision":"r2"}`))
		}
	})

	return log
}

func body(t *testing.T, raw string) map[string]any {
	t.Helper()

	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("request body %q is not JSON: %v", raw, err)
	}

	return out
}

func TestGetJSONKeysRowsBySlug(t *testing.T) {
	gridServer(t, nil)

	stdout, _, err := testutil.Execute(t, grid.NewCmd, "get", "g1", "--json", "revision,rows,columns")
	if err != nil {
		t.Fatalf("grid get returned error: %v", err)
	}

	var got struct {
		Revision string           `json:"revision"`
		Rows     []map[string]any `json:"rows"`
		Columns  []map[string]any `json:"columns"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, stdout)
	}

	if got.Revision != "r1" {
		t.Errorf("revision = %q, want r1", got.Revision)
	}
	if len(got.Rows) != 2 || got.Rows[0]["_id"] != "1" || got.Rows[0]["name"] != "bolt" {
		t.Errorf("rows = %v, want objects keyed by slug with _id", got.Rows)
	}
	if got.Rows[1]["_id"] != "2" || got.Rows[1]["ok"] != nil {
		t.Errorf("short row = %v, want numeric ID as text and padded cells", got.Rows[1])
	}
	if len(got.Columns) != 4 || got.Columns[1]["type"] != "number" {
		t.Errorf("columns = %v, want the schema with types", got.Columns)
	}
}

func TestGetRestoresNumbersSentAsText(t *testing.T) {
	gridServer(t, map[string]string{
		"GET /grids/g1": `{"id":"g1","title":"T","revision":"3","structure":{"columns":[
			{"slug":"qty","title":"Qty","type":"number"},{"slug":"name","title":"Name","type":"string"}]},
			"rows":[{"id":"1","row":["7","12"]}]}`,
	})

	stdout, _, err := testutil.Execute(t, grid.NewCmd, "get", "g1", "--json", "rows")
	if err != nil {
		t.Fatalf("grid get returned error: %v", err)
	}

	if !strings.Contains(stdout, `"qty": 7`) || !strings.Contains(stdout, `"name": "12"`) {
		t.Errorf("stdout = %s, want the number column as a number and the text column untouched", stdout)
	}
}

func TestGetSendsQuery(t *testing.T) {
	log := gridServer(t, nil)

	_, _, err := testutil.Execute(t, grid.NewCmd, "get", "g1",
		"--filter", "[qty] > 5", "--sort", "-qty", "--cols", "name,qty", "--rows", "1,2")
	if err != nil {
		t.Fatalf("grid get returned error: %v", err)
	}

	query := log.Last().Query
	for _, want := range []string{"filter=%5Bqty%5D+%3E+5", "sort=-qty", "only_cols=name%2Cqty", "only_rows=1%2C2"} {
		if !strings.Contains(query, want) {
			t.Errorf("query %q does not contain %q", query, want)
		}
	}
}

func TestGetCSVRoundTripsThroughRowAdd(t *testing.T) {
	gridServer(t, nil)

	stdout, _, err := testutil.Execute(t, grid.NewCmd, "get", "g1", "--format", "csv")
	if err != nil {
		t.Fatalf("grid get returned error: %v", err)
	}

	records, err := csv.NewReader(strings.NewReader(stdout)).ReadAll()
	if err != nil {
		t.Fatalf("output is not CSV: %v\n%s", err, stdout)
	}
	if got := strings.Join(records[0], ","); got != "_id,name,qty,ok,tags" {
		t.Errorf("header = %q, want _id then the slugs", got)
	}
	if got := strings.Join(records[1][:4], ","); got != "1,bolt,10,true" || records[1][4] != "a, b" {
		t.Errorf("first record = %v, want the row with its list cell joined", records[1])
	}

	log := gridServer(t, nil)
	_, _, err = testutil.ExecuteWithStdin(t, grid.NewCmd, stdout, "row", "add", "g1", "--rows-file", "-")
	if err != nil {
		t.Fatalf("row add of the exported CSV returned error: %v", err)
	}
	if got := log.Last().Path; got != "/grids/g1/rows" {
		t.Errorf("last request = %q, want the add-rows endpoint", got)
	}
}

func TestGetTable(t *testing.T) {
	gridServer(t, nil)

	stdout, _, err := testutil.Execute(t, grid.NewCmd, "get", "g1")
	if err != nil {
		t.Fatalf("grid get returned error: %v", err)
	}

	for _, want := range []string{"name", "bolt", "nut", "a, b", "revision r1"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout)
		}
	}
}

func TestGetQuietPrintsRowIDs(t *testing.T) {
	gridServer(t, nil)

	stdout, _, err := testutil.Execute(t, grid.NewCmd, "get", "g1", "--quiet")
	if err != nil {
		t.Fatalf("grid get returned error: %v", err)
	}

	if stdout != "1\n2\n" {
		t.Errorf("stdout = %q, want row IDs one per line", stdout)
	}
}

func TestList(t *testing.T) {
	log := gridServer(t, map[string]string{
		"GET /pages/12345/grids": `{"results":[{"id":"g1","title":"Stock","created_at":"2026-01-02T03:04:05Z"}],"next_cursor":""}`,
	})

	stdout, _, err := testutil.Execute(t, grid.NewCmd, "list", "users/me/notes", "--quiet")
	if err != nil {
		t.Fatalf("grid list returned error: %v", err)
	}

	if stdout != "g1\n" {
		t.Errorf("stdout = %q, want the table ID", stdout)
	}
	if log.At(0).Path != "/pages" {
		t.Errorf("first request = %q, want the slug lookup", log.At(0).Path)
	}
}

func TestRowAddCSVConvertsTypes(t *testing.T) {
	log := gridServer(t, map[string]string{
		"POST /grids/g1/rows": `{"revision":"r2","results":[{"id":"7","row":[]},{"id":"8","row":[]}]}`,
	})

	stdout, _, err := testutil.ExecuteWithStdin(t, grid.NewCmd,
		"name,qty,ok,tags\nwasher,5,true,\"x,y\"\nscrew,,,\n",
		"row", "add", "g1", "--rows-file", "-", "--quiet")
	if err != nil {
		t.Fatalf("row add returned error: %v", err)
	}

	if stdout != "7\n8\n" {
		t.Errorf("stdout = %q, want the new row IDs", stdout)
	}

	sent := body(t, log.Last().Body)
	if _, ok := sent["revision"]; ok {
		t.Errorf("body = %v, want no revision: the API does not use it", sent)
	}

	rows, _ := sent["rows"].([]any)
	first, _ := rows[0].(map[string]any)
	if first["qty"] != float64(5) || first["ok"] != true {
		t.Errorf("first row = %v, want qty 5 and ok true as typed values", first)
	}
	if tags, _ := first["tags"].([]any); len(tags) != 2 {
		t.Errorf("tags = %v, want a two-item list", first["tags"])
	}
	if second, _ := rows[1].(map[string]any); len(second) != 1 {
		t.Errorf("second row = %v, want empty CSV cells left out", second)
	}
}

func TestRowAddJSONIgnoresIDAndKeepsPositionZero(t *testing.T) {
	log := gridServer(t, nil)

	_, _, err := testutil.Execute(t, grid.NewCmd, "row", "add", "g1",
		"--rows", `[{"_id": "9", "name": "new"}]`, "--position", "0")
	if err != nil {
		t.Fatalf("row add returned error: %v", err)
	}

	sent := body(t, log.Last().Body)
	if pos, ok := sent["position"]; !ok || pos != float64(0) {
		t.Errorf("position = %v (present: %v), want an explicit 0", pos, ok)
	}
	rows, _ := sent["rows"].([]any)
	if row, _ := rows[0].(map[string]any); row["_id"] != nil || row["name"] != "new" {
		t.Errorf("row = %v, want _id dropped", row)
	}
}

func TestRowAddRejectsUnknownColumn(t *testing.T) {
	log := gridServer(t, nil)

	_, _, err := testutil.Execute(t, grid.NewCmd, "row", "add", "g1", "--rows", `[{"nmae": "x"}]`)

	exitErr, ok := errors.AsType[*wikierrors.ExitError](err)
	if !ok {
		t.Fatalf("error = %v, want an ExitError", err)
	}
	if !strings.Contains(exitErr.Message, `"nmae"`) || !strings.Contains(exitErr.Suggestion, "name, qty") {
		t.Errorf("error = %+v, want the bad slug and the valid ones", exitErr)
	}
	if log.Len() != 1 {
		t.Errorf("made %d requests, want only the read: nothing may be written", log.Len())
	}
}

func TestCellSetSingleConvertsValue(t *testing.T) {
	log := gridServer(t, nil)

	stdout, _, err := testutil.Execute(t, grid.NewCmd, "cell", "set", "g1", "2", "qty", "42", "--quiet")
	if err != nil {
		t.Fatalf("cell set returned error: %v", err)
	}
	if stdout != "r2\n" {
		t.Errorf("stdout = %q, want the new revision", stdout)
	}

	sent := body(t, log.Last().Body)
	cells, _ := sent["cells"].([]any)
	cell, _ := cells[0].(map[string]any)
	if cell["row_id"] != float64(2) || cell["column_slug"] != "qty" || cell["value"] != float64(42) {
		t.Errorf("cell = %v, want row_id 2 (number), qty, value 42 (number)", cell)
	}
}

func TestCellSetClearSendsNull(t *testing.T) {
	log := gridServer(t, nil)

	_, _, err := testutil.Execute(t, grid.NewCmd, "cell", "set", "g1", "1", "name", "--clear")
	if err != nil {
		t.Fatalf("cell set --clear returned error: %v", err)
	}

	if !strings.Contains(log.Last().Body, `"value":null`) {
		t.Errorf("body = %s, want an explicit null value", log.Last().Body)
	}
}

func TestCellSetBatch(t *testing.T) {
	log := gridServer(t, nil)

	_, _, err := testutil.Execute(t, grid.NewCmd, "cell", "set", "g1", "--cells",
		`[{"row_id": 1, "column": "qty", "value": 5}, {"row_id": "2", "column_slug": "name", "value": "x"}]`)
	if err != nil {
		t.Fatalf("cell set batch returned error: %v", err)
	}

	sent := body(t, log.Last().Body)
	if cells, _ := sent["cells"].([]any); len(cells) != 2 {
		t.Errorf("cells = %v, want both changes in one request", cells)
	}
}

func TestCellSetRejectsBadNumber(t *testing.T) {
	log := gridServer(t, nil)

	_, _, err := testutil.Execute(t, grid.NewCmd, "cell", "set", "g1", "1", "qty", "lots")
	if err == nil {
		t.Fatal("cell set accepted a non-number for a number column")
	}
	if log.Len() != 1 {
		t.Errorf("made %d requests, want only the read", log.Len())
	}
}

func TestRowRemoveSendsOnlyRowIDs(t *testing.T) {
	log := gridServer(t, nil)

	_, _, err := testutil.Execute(t, grid.NewCmd, "row", "remove", "g1", "1", "2")
	if err != nil {
		t.Fatalf("row remove returned error: %v", err)
	}

	if log.Len() != 1 || log.At(0).Method != http.MethodDelete {
		t.Fatalf("requests = %d, want a single DELETE with no prior read", log.Len())
	}

	if sent := body(t, log.At(0).Body); len(sent["row_ids"].([]any)) != 2 || len(sent) != 1 {
		t.Errorf("body = %v, want just the two row IDs", sent)
	}
}

func TestRowMoveNeedsDestination(t *testing.T) {
	gridServer(t, nil)

	_, _, err := testutil.Execute(t, grid.NewCmd, "row", "move", "g1", "1")
	if err == nil {
		t.Fatal("row move without --after or --position succeeded")
	}
}

func TestColumnAddFromSpec(t *testing.T) {
	log := gridServer(t, nil)

	_, _, err := testutil.Execute(t, grid.NewCmd, "column", "add", "g1",
		"--column", "price:number:Price", "--position", "1")
	if err != nil {
		t.Fatalf("column add returned error: %v", err)
	}

	sent := body(t, log.Last().Body)
	cols, _ := sent["columns"].([]any)
	col, _ := cols[0].(map[string]any)
	if col["slug"] != "price" || col["type"] != "number" || col["title"] != "Price" {
		t.Errorf("column = %v, want price/number/Price", col)
	}
	if required, ok := col["required"]; !ok || required != false {
		t.Errorf("column = %v, want an explicit required:false, which the API insists on", col)
	}
	if sent["position"] != float64(1) {
		t.Errorf("body = %v, want position 1", sent)
	}
}

func TestColumnAddFromStdinFile(t *testing.T) {
	log := gridServer(t, nil)

	_, _, err := testutil.ExecuteWithStdin(t, grid.NewCmd,
		`[{"slug":"status","title":"Status","type":"select","select_options":["open","done"]}]`,
		"column", "add", "g1", "--columns-file", "-")
	if err != nil {
		t.Fatalf("column add returned error: %v", err)
	}

	if !strings.Contains(log.Last().Body, `"select_options":["open","done"]`) {
		t.Errorf("body = %s, want the select options passed through", log.Last().Body)
	}
}

func TestColumnMoveAndRemove(t *testing.T) {
	log := gridServer(t, nil)

	if _, _, err := testutil.Execute(t, grid.NewCmd, "column", "move", "g1", "qty", "0"); err != nil {
		t.Fatalf("column move returned error: %v", err)
	}
	if got := log.Last(); got.Path != "/grids/g1/columns/move" || body(t, got.Body)["position"] != float64(0) {
		t.Errorf("move request = %s %s", got.Path, got.Body)
	}

	if _, _, err := testutil.Execute(t, grid.NewCmd, "column", "remove", "g1", "ok"); err != nil {
		t.Fatalf("column remove returned error: %v", err)
	}
	if got := log.Last(); got.Method != http.MethodDelete || !strings.Contains(got.Body, `"column_slugs":["ok"]`) {
		t.Errorf("remove request = %s %s", got.Method, got.Body)
	}
}

func TestCreateWithColumnsAndEmbed(t *testing.T) {
	log := gridServer(t, map[string]string{
		"POST /grids": `{"id":"g9","title":"Tasks","revision":"r1"}`,
	})

	stdout, _, err := testutil.Execute(t, grid.NewCmd, "create", "users/me/notes",
		"--title", "Tasks", "--column", "name:string", "--embed", "--json", "id,embedded,revision")
	if err != nil {
		t.Fatalf("grid create returned error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if got["id"] != "g9" || got["embedded"] != true || got["revision"] != "r2" {
		t.Errorf("output = %v, want id g9, embedded, and the revision after the columns", got)
	}

	paths := make([]string, log.Len())
	for i := range paths {
		paths[i] = log.At(i).Method + " " + log.At(i).Path
	}
	if !strings.Contains(log.At(1).Body, `"required":false`) {
		t.Errorf("columns body = %s, want an explicit required:false, which the API insists on", log.At(1).Body)
	}

	want := "POST /grids,POST /grids/g9/columns,GET /pages,POST /pages/12345/append-content"
	if strings.Join(paths, ",") != want {
		t.Errorf("requests = %v, want %s", paths, want)
	}
	if !strings.Contains(log.Last().Body, `{% wgrid id=\"g9\" %}`) {
		t.Errorf("append body = %s, want the wgrid tag", log.Last().Body)
	}
}

func TestCreateReportsTableWhenLaterStepFails(t *testing.T) {
	testutil.StubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/grids" {
			_, _ = w.Write([]byte(`{"id":"g9","title":"Tasks","revision":"r1"}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error_code":"VALIDATION_ERROR","debug_message":"bad column"}`))
	})

	_, _, err := testutil.Execute(t, grid.NewCmd, "create", "users/me/notes",
		"--title", "Tasks", "--column", "name:string")

	exitErr, ok := errors.AsType[*wikierrors.ExitError](err)
	if !ok {
		t.Fatalf("error = %v, want an ExitError", err)
	}
	if !strings.Contains(exitErr.Message, "g9 was created") || !strings.Contains(exitErr.Suggestion, "column add g9") {
		t.Errorf("error = %+v, want the created table ID and how to finish", exitErr)
	}
}

func TestDelete(t *testing.T) {
	log := gridServer(t, nil)

	stdout, _, err := testutil.Execute(t, grid.NewCmd, "delete", "g1")
	if err != nil {
		t.Fatalf("grid delete returned error: %v", err)
	}

	if log.Len() != 1 || log.Last().Method != http.MethodDelete || !strings.Contains(stdout, "Deleted table g1") {
		t.Errorf("requests = %d, last %s, stdout %q; want a single DELETE", log.Len(), log.Last().Method, stdout)
	}
}

func TestUpdateSendsTitleAndSort(t *testing.T) {
	log := gridServer(t, nil)

	_, _, err := testutil.Execute(t, grid.NewCmd, "update", "g1", "--title", "New", "--sort", "name,-qty")
	if err != nil {
		t.Fatalf("grid update returned error: %v", err)
	}

	got := log.Last()
	if got.Method != http.MethodPost || got.Path != "/grids/g1" {
		t.Errorf("request = %s %s, want POST /grids/g1", got.Method, got.Path)
	}
	if !strings.Contains(got.Body, `"default_sort":[{"name":"asc"},{"qty":"desc"}]`) {
		t.Errorf("body = %s, want the sort", got.Body)
	}
}

func TestUpdateNeedsAChange(t *testing.T) {
	gridServer(t, nil)

	if _, _, err := testutil.Execute(t, grid.NewCmd, "update", "g1"); err == nil {
		t.Fatal("grid update with no flags succeeded")
	}
}

func TestCloneWaitsForOperation(t *testing.T) {
	log := gridServer(t, map[string]string{
		"POST /grids/g1/clone":      `{"operation":{"type":"clone_grid","id":"op1"},"status_url":"https://api.wiki.yandex.net/v1/operations/clone/op1"}`,
		"GET /operations/clone/op1": `{"status":"success","result":{"grid_id":"g2","page":{"id":1,"slug":"users/me/archive"}}}`,
	})

	stdout, _, err := testutil.Execute(t, grid.NewCmd, "clone", "g1", "--to", "users/me/archive", "--with-data")
	if err != nil {
		t.Fatalf("grid clone returned error: %v", err)
	}

	if !strings.Contains(log.At(0).Body, `"with_data":true`) {
		t.Errorf("clone body = %s, want with_data", log.At(0).Body)
	}
	if got := log.Last().Path; got != "/operations/clone/op1" {
		t.Errorf("last request = %q, want the operation poll through the client's own base URL", got)
	}
	if !strings.Contains(stdout, "table g2") {
		t.Errorf("stdout = %q, want the ID of the copy", stdout)
	}
}

func TestConflictHasOwnExitCode(t *testing.T) {
	testutil.StubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(gridJSON))
			return
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error_code":"CONFLICT","debug_message":"stale revision"}`))
	})

	_, _, err := testutil.Execute(t, grid.NewCmd, "cell", "set", "g1", "1", "name", "x")

	exitErr, ok := errors.AsType[*wikierrors.ExitError](err)
	if !ok || exitErr.ExitCode != wikierrors.ExitConflict {
		t.Fatalf("error = %v, want exit code %d", err, wikierrors.ExitConflict)
	}
	if !strings.Contains(exitErr.Suggestion, "ywiki grid get") {
		t.Errorf("suggestion = %q, want a table-specific recovery", exitErr.Suggestion)
	}
}
