package models

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"sort"
)

// GenerateAttributesHash generate属性组合的哈希值
// 用于快速Query相同属性组合的Inventory
func GenerateAttributesHash(attrs map[string]string) string {
	if len(attrs) == 0 {
		return ""
	}
	
	// 1. 提取所有键并排序（确保相同属性组合generate相同哈希）
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	// 2. 按键排序后构建JSON字符串
	sortedAttrs := make(map[string]string)
	for _, k := range keys {
		sortedAttrs[k] = attrs[k]
	}
	
	jsonData, _ := json.Marshal(sortedAttrs)
	
	// 3. 计算MD5哈希
	hash := md5.Sum(jsonData)
	return hex.EncodeToString(hash[:])
}

// AttributesMatch 检查两个属性组合是否匹配
func AttributesMatch(attrs1, attrs2 map[string]string) bool {
	if len(attrs1) != len(attrs2) {
		return false
	}
	
	for k, v1 := range attrs1 {
		v2, ok := attrs2[k]
		if !ok || v1 != v2 {
			return false
		}
	}
	
	return true
}

// NormalizeAttributes 标准化属性（去除空值、trim空格等）
func NormalizeAttributes(attrs map[string]string) map[string]string {
	normalized := make(map[string]string)
	for k, v := range attrs {
		if k != "" && v != "" {
			normalized[k] = v
		}
	}
	return normalized
}
