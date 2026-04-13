package opensearch

import (
	"testing"
)

func TestKNNQuery_Validation(t *testing.T) {
	tests := []struct {
		name    string
		query   KNNQuery
		wantErr bool
	}{
		{
			name: "valid query",
			query: KNNQuery{
				Vector: []float32{0.1, 0.2, 0.3},
				Field:  "content_vector",
				K:      10,
			},
			wantErr: false,
		},
		{
			name: "empty vector",
			query: KNNQuery{
				Vector: []float32{},
				Field:  "content_vector",
				K:      10,
			},
			wantErr: false,
		},
		{
			name: "nil vector",
			query: KNNQuery{
				Vector: nil,
				Field:  "content_vector",
				K:      10,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 验证查询结构可以被正确构建
			if tt.query.Field == "" {
				t.Error("Field should not be empty")
			}
			if tt.query.K <= 0 {
				t.Error("K should be positive")
			}
		})
	}
}

func TestHybridQuery_Validation(t *testing.T) {
	tests := []struct {
		name    string
		query   HybridQuery
		wantErr bool
	}{
		{
			name: "valid query",
			query: HybridQuery{
				Query:       "test query",
				Vector:      []float32{0.1, 0.2, 0.3},
				VectorField: "content_vector",
				K:           10,
			},
			wantErr: false,
		},
		{
			name: "empty vector",
			query: HybridQuery{
				Query:       "test query",
				Vector:      []float32{},
				VectorField: "content_vector",
				K:           10,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 验证查询结构可以被正确构建
			if tt.query.Query == "" {
				t.Error("Query should not be empty")
			}
			if tt.query.VectorField == "" {
				t.Error("VectorField should not be empty")
			}
			if tt.query.K <= 0 {
				t.Error("K should be positive")
			}
		})
	}
}

func TestClient_buildKNNSearchBody(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name     string
		query    *KNNQuery
		validate func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "basic knn query",
			query: &KNNQuery{
				Vector: []float32{0.1, 0.2, 0.3},
				Field:  "content_vector",
				K:      10,
			},
			validate: func(t *testing.T, body map[string]interface{}) {
				if body["size"] != 10 {
					t.Errorf("size = %v, want 10", body["size"])
				}
				knn, ok := body["knn"].(map[string]interface{})
				if !ok {
					t.Fatal("knn should be a map")
				}
				if _, ok := knn["content_vector"]; !ok {
					t.Error("knn should contain content_vector")
				}
			},
		},
		{
			name: "knn query with filters",
			query: &KNNQuery{
				Vector: []float32{0.1, 0.2, 0.3},
				Field:  "content_vector",
				K:      10,
				Filters: map[string]interface{}{
					"file_type": "pdf",
				},
			},
			validate: func(t *testing.T, body map[string]interface{}) {
				knn, ok := body["knn"].(map[string]interface{})
				if !ok {
					t.Fatal("knn should be a map")
				}
				filter, ok := knn["filter"]
				if !ok {
					t.Error("knn should contain filter")
				}
				filterArray, ok := filter.([]map[string]interface{})
				if !ok {
					t.Fatal("filter should be an array")
				}
				if len(filterArray) == 0 {
					t.Error("filter should not be empty")
				}
			},
		},
		{
			name: "image vector search",
			query: &KNNQuery{
				Vector: []float32{0.1, 0.2, 0.3},
				Field:  "image_vector",
				K:      5,
			},
			validate: func(t *testing.T, body map[string]interface{}) {
				if body["size"] != 5 {
					t.Errorf("size = %v, want 5", body["size"])
				}
				knn, ok := body["knn"].(map[string]interface{})
				if !ok {
					t.Fatal("knn should be a map")
				}
				if _, ok := knn["image_vector"]; !ok {
					t.Error("knn should contain image_vector")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := client.buildKNNSearchBody(tt.query)
			tt.validate(t, body)
		})
	}
}

func TestClient_buildHybridSearchBody(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name     string
		query    *HybridQuery
		validate func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "basic hybrid query",
			query: &HybridQuery{
				Query:       "test query",
				Vector:      []float32{0.1, 0.2, 0.3},
				VectorField: "content_vector",
				K:           10,
			},
			validate: func(t *testing.T, body map[string]interface{}) {
				if body["size"] != 10 {
					t.Errorf("size = %v, want 10", body["size"])
				}
				query, ok := body["query"].(map[string]interface{})
				if !ok {
					t.Fatal("query should be a map")
				}
				if _, ok := query["bool"]; !ok {
					t.Error("query should contain bool")
				}
				if _, ok := body["knn"]; !ok {
					t.Error("body should contain knn")
				}
			},
		},
		{
			name: "hybrid query with filters",
			query: &HybridQuery{
				Query:       "test query",
				Vector:      []float32{0.1, 0.2, 0.3},
				VectorField: "content_vector",
				K:           10,
				Filters: map[string]interface{}{
					"file_type": "pdf",
				},
			},
			validate: func(t *testing.T, body map[string]interface{}) {
				knn, ok := body["knn"].(map[string]interface{})
				if !ok {
					t.Fatal("knn should be a map")
				}
				filter, ok := knn["filter"]
				if !ok {
					t.Error("knn should contain filter")
				}
				filterArray, ok := filter.([]map[string]interface{})
				if !ok {
					t.Fatal("filter should be an array")
				}
				if len(filterArray) == 0 {
					t.Error("filter should not be empty")
				}
			},
		},
		{
			name: "hybrid query without vector",
			query: &HybridQuery{
				Query:       "test query",
				Vector:      nil,
				VectorField: "content_vector",
				K:           10,
			},
			validate: func(t *testing.T, body map[string]interface{}) {
				if _, ok := body["knn"]; ok {
					t.Error("body should not contain knn when vector is nil")
				}
				query, ok := body["query"].(map[string]interface{})
				if !ok {
					t.Fatal("query should be a map")
				}
				if _, ok := query["bool"]; !ok {
					t.Error("query should contain bool")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := client.buildHybridSearchBody(tt.query)
			tt.validate(t, body)
		})
	}
}

func TestFileMapping_VectorFields(t *testing.T) {
	mapping := FileMapping()

	properties, ok := mapping["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("properties should be a map")
	}

	// 验证 content_vector 字段
	contentVector, ok := properties["content_vector"].(map[string]interface{})
	if !ok {
		t.Fatal("content_vector field should exist")
	}
	if contentVector["type"] != "dense_vector" {
		t.Errorf("content_vector type = %v, want dense_vector", contentVector["type"])
	}
	if contentVector["dims"] != 1536 {
		t.Errorf("content_vector dims = %v, want 1536", contentVector["dims"])
	}
	if contentVector["index"] != true {
		t.Error("content_vector should be indexed")
	}
	if contentVector["similarity"] != "cosine" {
		t.Errorf("content_vector similarity = %v, want cosine", contentVector["similarity"])
	}

	// 验证 image_vector 字段
	imageVector, ok := properties["image_vector"].(map[string]interface{})
	if !ok {
		t.Fatal("image_vector field should exist")
	}
	if imageVector["type"] != "dense_vector" {
		t.Errorf("image_vector type = %v, want dense_vector", imageVector["type"])
	}
	if imageVector["dims"] != 512 {
		t.Errorf("image_vector dims = %v, want 512", imageVector["dims"])
	}
	if imageVector["index"] != true {
		t.Error("image_vector should be indexed")
	}
	if imageVector["similarity"] != "cosine" {
		t.Errorf("image_vector similarity = %v, want cosine", imageVector["similarity"])
	}
}

func TestSearchHit_VectorFields(t *testing.T) {
	// 验证 SearchHit 可以包含向量字段
	hit := SearchHit{
		ID:    "test-id",
		Score: 0.95,
		Source: map[string]interface{}{
			"filename":       "test.pdf",
			"content_vector": []float32{0.1, 0.2, 0.3},
		},
	}

	if hit.ID != "test-id" {
		t.Errorf("ID = %v, want test-id", hit.ID)
	}
	if hit.Score != 0.95 {
		t.Errorf("Score = %v, want 0.95", hit.Score)
	}
	if hit.Source["filename"] != "test.pdf" {
		t.Error("Source should contain filename")
	}
}
