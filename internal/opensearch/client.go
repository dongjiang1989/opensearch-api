package opensearch

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/opensearch-project/opensearch-go/v3"
	"github.com/opensearch-project/opensearch-go/v3/opensearchapi"
	"go.uber.org/zap"

	"github.com/dongjiang1989/opensearch-api/internal/config"
)

// Client OpenSearch 客户端封装
type Client struct {
	client *opensearchapi.Client
	config *config.OpenSearchConfig
	logger *zap.Logger
}

// NewClient 创建新的 OpenSearch 客户端
func NewClient(cfg *config.OpenSearchConfig, logger *zap.Logger) (*Client, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: !cfg.Secure,
	}

	apiClient, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{
			Addresses: []string{cfg.URL()},
			Username:  cfg.Username,
			Password:  cfg.Password,
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create opensearch client: %w", err)
	}

	return &Client{
		client: apiClient,
		config: cfg,
		logger: logger,
	}, nil
}

// IndexName 生成租户索引名称
func (c *Client) IndexName(tenantID string) string {
	return fmt.Sprintf("%s_%s_files", c.config.IndexPrefix, tenantID)
}

// AliasName 生成租户索引别名
func (c *Client) AliasName(tenantID string) string {
	return fmt.Sprintf("%s_%s", c.config.IndexPrefix, tenantID)
}

// CreateIndex 创建索引
func (c *Client) CreateIndex(ctx context.Context, tenantID string, mapping map[string]interface{}) error {
	indexName := c.IndexName(tenantID)

	// 检查索引是否已存在
	exists, err := c.IndexExists(ctx, indexName)
	if err != nil {
		return err
	}
	if exists {
		c.logger.Info("index already exists", zap.String("index", indexName))
		return nil
	}

	// 创建索引
	body := map[string]interface{}{
		"settings": map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		"mappings": mapping,
	}

	req := opensearchapi.IndicesCreateReq{
		Index: indexName,
		Body:  bytes.NewReader(mustMarshal(body)),
	}

	res, err := c.client.Indices.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	if res.Inspect().Response.IsError() {
		return fmt.Errorf("failed to create index %s: %s", indexName, res.Inspect().Response.String())
	}

	c.logger.Info("index created", zap.String("index", indexName))
	return nil
}

// IndexExists 检查索引是否存在
func (c *Client) IndexExists(ctx context.Context, indexName string) (bool, error) {
	req := opensearchapi.IndicesExistsReq{
		Indices: []string{indexName},
	}

	res, err := c.client.Indices.Exists(ctx, req)
	if err != nil {
		return false, fmt.Errorf("failed to check index exists: %w", err)
	}

	if res.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return false, fmt.Errorf("failed to check index exists: %s", string(body))
	}

	return res.StatusCode == http.StatusOK, nil
}

// DeleteIndex 删除索引
func (c *Client) DeleteIndex(ctx context.Context, tenantID string) error {
	indexName := c.IndexName(tenantID)

	req := opensearchapi.IndicesDeleteReq{
		Indices: []string{indexName},
	}

	res, err := c.client.Indices.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete index: %w", err)
	}

	inspect := res.Inspect()
	if inspect.Response.IsError() && inspect.Response.StatusCode != http.StatusNotFound {
		return fmt.Errorf("failed to delete index %s: %s", indexName, inspect.Response.String())
	}

	c.logger.Info("index deleted", zap.String("index", indexName))
	return nil
}

// IndexDocument 索引文档
func (c *Client) IndexDocument(ctx context.Context, tenantID, docID string, doc map[string]interface{}) error {
	indexName := c.IndexName(tenantID)

	req := opensearchapi.IndexReq{
		Index:      indexName,
		DocumentID: docID,
		Body:       bytes.NewReader(mustMarshal(doc)),
		Params: opensearchapi.IndexParams{
			Refresh: "true",
		},
	}

	res, err := c.client.Index(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to index document: %w", err)
	}

	if res.Inspect().Response.IsError() {
		return fmt.Errorf("failed to index document: %s", res.Inspect().Response.String())
	}

	return nil
}

// GetDocument 获取文档
func (c *Client) GetDocument(ctx context.Context, tenantID, docID string) (map[string]interface{}, error) {
	indexName := c.IndexName(tenantID)

	req := opensearchapi.DocumentGetReq{
		Index:      indexName,
		DocumentID: docID,
	}

	res, err := c.client.Document.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	inspect := res.Inspect()
	if inspect.Response.IsError() {
		if inspect.Response.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get document: %s", inspect.Response.String())
	}

	var source map[string]interface{}
	if len(res.Source) > 0 {
		if err := json.Unmarshal(res.Source, &source); err != nil {
			return nil, fmt.Errorf("failed to parse document source: %w", err)
		}
	}

	return source, nil
}

// DeleteDocument 删除文档
func (c *Client) DeleteDocument(ctx context.Context, tenantID, docID string) error {
	indexName := c.IndexName(tenantID)

	req := opensearchapi.DocumentDeleteReq{
		Index:      indexName,
		DocumentID: docID,
		Params: opensearchapi.DocumentDeleteParams{
			Refresh: "true",
		},
	}

	res, err := c.client.Document.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	inspect := res.Inspect()
	if inspect.Response.IsError() && inspect.Response.StatusCode != http.StatusNotFound {
		return fmt.Errorf("failed to delete document: %s", inspect.Response.String())
	}

	return nil
}

// Health 检查 OpenSearch 健康状态
func (c *Client) Health(ctx context.Context) (map[string]interface{}, error) {
	var req *opensearchapi.ClusterHealthReq

	res, err := c.client.Cluster.Health(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster health: %w", err)
	}

	if res.Inspect().Response.IsError() {
		return nil, fmt.Errorf("failed to get cluster health: %s", res.Inspect().Response.String())
	}

	// 直接 marshal 响应结构体
	data, err := json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal health response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse health response: %w", err)
	}

	return result, nil
}

// Ping 检查 OpenSearch 连接
func (c *Client) Ping(ctx context.Context) error {
	res, err := c.client.Info(ctx, nil)
	if err != nil {
		return err
	}

	inspect := res.Inspect()
	if inspect.Response.IsError() {
		return fmt.Errorf("ping failed: %s", inspect.Response.String())
	}

	return nil
}

// Search 执行搜索
func (c *Client) Search(ctx context.Context, tenantID string, query *SearchQuery) (*SearchResult, error) {
	indexName := c.IndexName(tenantID)

	// 构建搜索体
	body := c.buildSearchBody(query)

	req := &opensearchapi.SearchReq{
		Indices: []string{indexName},
		Body:    bytes.NewReader(mustMarshal(body)),
	}

	res, err := c.client.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	if res.Inspect().Response.IsError() {
		return nil, fmt.Errorf("search failed: %s", res.Inspect().Response.String())
	}

	// 直接从响应结构体中解析数据
	hitArray := res.Hits.Hits
	var hits []SearchHit
	for _, h := range hitArray {
		var source map[string]interface{}
		if len(h.Source) > 0 {
			if err := json.Unmarshal(h.Source, &source); err != nil {
				return nil, fmt.Errorf("failed to parse hit source: %w", err)
			}
		}

		hits = append(hits, SearchHit{
			ID:     h.ID,
			Score:  float64(h.Score),
			Source: source,
		})
	}

	return &SearchResult{
		Total: res.Hits.Total.Value,
		Hits:  hits,
		Took:  res.Took,
	}, nil
}

// buildSearchBody 构建搜索请求体
func (c *Client) buildSearchBody(query *SearchQuery) map[string]interface{} {
	body := map[string]interface{}{
		"from": query.From,
		"size": query.Size,
	}

	if query.Query != "" || len(query.Filters) > 0 {
		body["query"] = c.buildQuery(query)
	}

	if len(query.Sort) > 0 {
		body["sort"] = query.Sort
	}

	if query.Highlight != nil {
		body["highlight"] = query.Highlight
	}

	return body
}

// buildQuery 构建查询 DSL
func (c *Client) buildQuery(query *SearchQuery) map[string]interface{} {
	if query.Query == "" && len(query.Filters) == 0 {
		return map[string]interface{}{
			"match_all": map[string]interface{}{},
		}
	}

	if query.Query != "" && len(query.Filters) == 0 {
		return map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  query.Query,
				"fields": []string{"content", "filename", "description", "tags"},
			},
		}
	}

	if query.Query == "" && len(query.Filters) > 0 {
		filterArray := make([]map[string]interface{}, 0, len(query.Filters))
		for key, value := range query.Filters {
			filterArray = append(filterArray, map[string]interface{}{
				"term": map[string]interface{}{
					key: value,
				},
			})
		}
		return map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": filterArray,
			},
		}
	}

	filterArray := make([]map[string]interface{}, 0, len(query.Filters))
	for key, value := range query.Filters {
		filterArray = append(filterArray, map[string]interface{}{
			"term": map[string]interface{}{
				key: value,
			},
		})
	}

	return map[string]interface{}{
		"bool": map[string]interface{}{
			"must": map[string]interface{}{
				"multi_match": map[string]interface{}{
					"query":  query.Query,
					"fields": []string{"content", "filename", "description", "tags"},
				},
			},
			"filter": filterArray,
		},
	}
}

// parseSearchResult 解析搜索结果
func (c *Client) parseSearchResult(result map[string]interface{}) (*SearchResult, error) {
	hitsSection, ok := result["hits"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid hits section")
	}

	total, _ := c.parseTotal(hitsSection)

	hitArray, _ := hitsSection["hits"].([]interface{})
	var hits []SearchHit
	for _, h := range hitArray {
		hitMap, _ := h.(map[string]interface{})
		source, _ := hitMap["_source"].(map[string]interface{})
		highlight, _ := hitMap["highlight"].(map[string]interface{})

		var score float64
		if s, ok := hitMap["_score"].(float64); ok {
			score = s
		}

		hits = append(hits, SearchHit{
			ID:        getString(hitMap, "_id"),
			Score:     score,
			Source:    source,
			Highlight: highlight,
		})
	}

	took := 0
	if t, ok := result["took"].(float64); ok {
		took = int(t)
	}

	return &SearchResult{
		Total:    total,
		Hits:     hits,
		Took:     took,
		Metadata: result,
	}, nil
}

func (c *Client) parseTotal(hitsSection map[string]interface{}) (int, error) {
	total := 0
	switch v := hitsSection["total"].(type) {
	case float64:
		total = int(v)
	case map[string]interface{}:
		if value, ok := v["value"].(float64); ok {
			total = int(value)
		}
	}
	return total, nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// Aggregate 执行聚合查询
func (c *Client) Aggregate(ctx context.Context, tenantID, fieldName string) (map[string]int64, error) {
	indexName := c.IndexName(tenantID)

	body := map[string]interface{}{
		"size": 0,
		"aggs": map[string]interface{}{
			fieldName: map[string]interface{}{
				"terms": map[string]interface{}{
					"field": fieldName,
					"size":  100,
				},
			},
		},
	}

	req := &opensearchapi.SearchReq{
		Indices: []string{indexName},
		Body:    bytes.NewReader(mustMarshal(body)),
	}

	res, err := c.client.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate: %w", err)
	}

	if res.Inspect().Response.IsError() {
		return nil, fmt.Errorf("aggregation failed: %s", res.Inspect().Response.String())
	}

	// 解析聚合结果
	data, err := json.Marshal(res)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal aggregation response: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse aggregation: %w", err)
	}

	aggs, ok := result["aggregations"].(map[string]interface{})
	if !ok {
		return map[string]int64{}, nil
	}

	fieldAgg, ok := aggs[fieldName].(map[string]interface{})
	if !ok {
		return map[string]int64{}, nil
	}

	buckets, ok := fieldAgg["buckets"].([]interface{})
	if !ok {
		return map[string]int64{}, nil
	}

	resultMap := make(map[string]int64)
	for _, b := range buckets {
		bucket, _ := b.(map[string]interface{})
		key := getString(bucket, "key")
		docCount := int64(0)
		if count, ok := bucket["doc_count"].(float64); ok {
			docCount = int64(count)
		}
		resultMap[key] = docCount
	}

	return resultMap, nil
}

// Count 统计文档数量
func (c *Client) Count(ctx context.Context, tenantID string) (int64, error) {
	indexName := c.IndexName(tenantID)

	req := &opensearchapi.IndicesCountReq{
		Indices: []string{indexName},
	}

	res, err := c.client.Indices.Count(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("failed to count: %w", err)
	}

	if res.Inspect().Response.IsError() {
		return 0, fmt.Errorf("count failed: %s", res.Inspect().Response.String())
	}

	return int64(res.Count), nil
}

// Refresh 刷新索引
func (c *Client) Refresh(ctx context.Context, tenantID string) error {
	indexName := c.IndexName(tenantID)

	req := &opensearchapi.IndicesRefreshReq{
		Indices: []string{indexName},
	}

	res, err := c.client.Indices.Refresh(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to refresh: %w", err)
	}

	if res.Inspect().Response.IsError() {
		return fmt.Errorf("refresh failed: %s", res.Inspect().Response.String())
	}

	return nil
}

// BulkIndex 批量索引
func (c *Client) BulkIndex(ctx context.Context, tenantID string, docs []BulkDoc) error {
	indexName := c.IndexName(tenantID)

	var buf bytes.Buffer
	for _, doc := range docs {
		buf.WriteString(`{"index":{"_id":"`)
		buf.WriteString(doc.ID)
		buf.WriteString(`"}}`)
		buf.WriteByte('\n')

		docBytes, _ := json.Marshal(doc.Source)
		buf.Write(docBytes)
		buf.WriteByte('\n')
	}

	req := opensearchapi.BulkReq{
		Index: indexName,
		Body:  bytes.NewReader(buf.Bytes()),
		Params: opensearchapi.BulkParams{
			Refresh: "true",
		},
	}

	res, err := c.client.Bulk(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to bulk: %w", err)
	}

	if res.Inspect().Response.IsError() {
		return fmt.Errorf("bulk failed: %s", res.Inspect().Response.String())
	}

	return nil
}

func mustMarshal(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}
