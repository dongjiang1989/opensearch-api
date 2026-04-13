package opensearch

// SearchQuery 搜索查询参数
type SearchQuery struct {
	Query     string                 // 全文搜索查询
	Filters   map[string]interface{} // 过滤条件
	From      int                    // 起始位置
	Size      int                    // 返回数量
	Sort      []map[string]interface{} // 排序规则
	Highlight map[string]interface{} // 高亮配置
}

// KNNQuery KNN 向量搜索查询参数
type KNNQuery struct {
	Vector     []float32              // 查询向量
	Field      string                 // 向量字段名 (content_vector, image_vector)
	K          int                    // 返回结果数量
	Filters    map[string]interface{} // 过滤条件（可选）
}

// HybridQuery 混合搜索查询参数（结合文本和向量搜索）
type HybridQuery struct {
	Query      string                 // 文本查询
	Vector     []float32              // 查询向量
	VectorField string                // 向量字段名
	K          int                    // 返回结果数量
	Filters    map[string]interface{} // 过滤条件
}

// SearchResult 搜索结果
type SearchResult struct {
	Total    int                      // 总数
	Hits     []SearchHit              // 命中结果
	Took     int                      // 耗时 (ms)
	Metadata map[string]interface{}   // 元数据
}

// SearchHit 单次命中结果
type SearchHit struct {
	ID        string                 // 文档 ID
	Score     float64                // 评分
	Source    map[string]interface{} // 文档源数据
	Highlight map[string]interface{} // 高亮片段
}

// BulkDoc 批量索引文档结构
type BulkDoc struct {
	ID     string
	Source map[string]interface{}
}
