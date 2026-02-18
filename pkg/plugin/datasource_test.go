package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TestQueryData(t *testing.T) {
	ds := NewDatasource(&DuckDBDriver{Initialized: false})
	_, err := ds.NewDatasource(context.Background(), backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"path":""}`),
	})

	resp, err := ds.QueryData(
		context.Background(),
		&backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{},
			},
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: json.RawMessage(`{"rawSql": "from duckdb_settings();"}`)},
			},
		},
	)
	if err != nil {
		t.Error(err)
	}
	if len(resp.Responses) != 1 {
		t.Fatal("QueryData must return a response")
	}
}

func TestCustomUserAgent(t *testing.T) {
	ds := NewDatasource(&DuckDBDriver{Initialized: false})
	_, err := ds.NewDatasource(context.Background(), backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"path":""}`),
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := ds.QueryData(
		context.Background(),
		&backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{},
			},
			Queries: []backend.DataQuery{
				{RefID: "A", JSON: json.RawMessage(`{"rawSql": "PRAGMA user_agent;"}`)},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Responses) != 1 {
		t.Fatal("expected 1 response")
	}
	for _, r := range resp.Responses {
		if r.Error != nil {
			t.Fatal(r.Error)
		}
		if len(r.Frames) == 0 || r.Frames[0].Fields[0].Len() == 0 {
			t.Fatal("expected non-empty result")
		}
		val := r.Frames[0].Fields[0].At(0)
		userAgent := fmt.Sprintf("%v", val)
		if sp, ok := val.(*string); ok && sp != nil {
			userAgent = *sp
		}
		if !strings.Contains(userAgent, "grafana") {
			t.Errorf("expected user_agent to contain 'grafana', got: %s", userAgent)
		}
		t.Logf("user_agent: %s", userAgent)
	}
}

func TestMultipleConcurrentRequests(t *testing.T) {
	ds := NewDatasource(&DuckDBDriver{Initialized: false})
	ctx := context.Background()
	_, err := ds.NewDatasource(context.Background(), backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"path":""}`),
	})
	if err != nil {
		t.Error(err)
	}

	var wg sync.WaitGroup
	numQueries := 23
	results := make(chan error, numQueries)

	for i := 0; i < numQueries; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := ds.QueryData(
				ctx,
				&backend.QueryDataRequest{
					PluginContext: backend.PluginContext{
						DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{},
					},
					Queries: []backend.DataQuery{
						{RefID: fmt.Sprintf("tempvar"), JSON: json.RawMessage(fmt.Sprintf(`{"rawSql": "SELECT cos(%d);"}`, i))},
					},
				},
			)
			if err != nil {
				results <- err
				return
			}
			if len(resp.Responses) != 1 {
				results <- fmt.Errorf("expected 1 response per request, got %d", len(resp.Responses))
				return
			}
			for _, _resp := range resp.Responses {
				if _resp.Error != nil {
					results <- _resp.Error
					return
				}
			}

			results <- nil
		}()
	}

	wg.Wait()
	close(results)

	for err := range results {
		if err != nil {
			t.Errorf("Query failed: %v", err)
		}
	}
}

func TestMultipleQueriesRequest(t *testing.T) {
	numQueries := 23
	ds := NewDatasource(&DuckDBDriver{Initialized: false})
	ctx := context.Background()
	_, err := ds.NewDatasource(context.Background(), backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"path":""}`),
	})
	if err != nil {
		t.Error(err)
	}

	queries := []backend.DataQuery{}
	for i := 0; i < numQueries; i++ {
		queries = append(queries, backend.DataQuery{RefID: fmt.Sprintf("Q%d", i), JSON: json.RawMessage(fmt.Sprintf(`{"rawSql": "SELECT %d;"}`, i))})
	}

	resp, err := ds.QueryData(
		ctx,
		&backend.QueryDataRequest{
			PluginContext: backend.PluginContext{
				DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{},
			},
			Queries: queries,
		},
	)
	if err != nil {
		t.Error(err)
	}
	if len(resp.Responses) != numQueries {
		t.Errorf("expected %d responses, got %d", numQueries, len(resp.Responses))
	}
	for _, _resp := range resp.Responses {
		if _resp.Error != nil {
			t.Error(_resp.Error)
		}
	}
}
