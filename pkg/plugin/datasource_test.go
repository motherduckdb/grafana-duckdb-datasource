package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TestQueryData(t *testing.T) {
	ds := NewDatasource(&DuckDBDriver{HasSetMotherDuckToken: false})
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
	fmt.Printf("%+v\n", resp.Responses)
}

func TestMultipleConcurrentRequests(t *testing.T) {
	ds := NewDatasource(&DuckDBDriver{HasSetMotherDuckToken: false})
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
	ds := NewDatasource(&DuckDBDriver{HasSetMotherDuckToken: false})
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
