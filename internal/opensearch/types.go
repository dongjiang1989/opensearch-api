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
