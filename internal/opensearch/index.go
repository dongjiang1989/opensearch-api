package opensearch

import (
	"context"
)

// FileMapping 返回文件索引的默认映射
func FileMapping() map[string]interface{} {
	return map[string]interface{}{
		"properties": map[string]interface{}{
			"filename": map[string]interface{}{
				"type":     "text",
				"analyzer": "standard",
				"fields": map[string]interface{}{
					"keyword": map[string]interface{}{
						"type": "keyword",
					},
				},
			},
			"content": map[string]interface{}{
				"type":     "text",
				"analyzer": "standard",
			},
			"content_type": map[string]interface{}{
				"type": "keyword",
			},
			"file_type": map[string]interface{}{
				"type": "keyword",
			},
			"file_size": map[string]interface{}{
				"type": "long",
			},
			"description": map[string]interface{}{
				"type":     "text",
				"analyzer": "standard",
			},
			"tags": map[string]interface{}{
				"type": "keyword",
			},
			"metadata": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"width":      map[string]interface{}{"type": "integer"},
					"height":     map[string]interface{}{"type": "integer"},
					"duration":   map[string]interface{}{"type": "float"},
					"pages":      map[string]interface{}{"type": "integer"},
					"author":     map[string]interface{}{"type": "text"},
					"created_at": map[string]interface{}{"type": "date"},
				},
			},
			"storage_path": map[string]interface{}{
				"type": "keyword",
			},
			"tenant_id": map[string]interface{}{
				"type": "keyword",
			},
			"created_at": map[string]interface{}{
				"type": "date",
			},
			"updated_at": map[string]interface{}{
				"type": "date",
			},
		},
	}
}

// CreateIndexWithMapping 为租户创建带映射的索引
func (c *Client) CreateIndexWithMapping(ctx context.Context, tenantID string) error {
	return c.CreateIndex(ctx, tenantID, FileMapping())
}
