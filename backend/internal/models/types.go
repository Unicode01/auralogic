package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSON 自定义JSON类型，用于GORM存储JSON数据
type JSON string

// Value 实现driver.Valuer接口
func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}

// Scan 实现sql.Scanner接口
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = ""
		return nil
	}
	
	switch v := value.(type) {
	case []byte:
		*j = JSON(v)
	case string:
		*j = JSON(v)
	default:
		return errors.New("failed to scan JSON value")
	}
	
	return nil
}

// MarshalJSON 实现json.Marshaler接口
func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

// UnmarshalJSON 实现json.Unmarshaler接口
func (j *JSON) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	*j = JSON(data)
	return nil
}

// String 转换为字符串
func (j JSON) String() string {
	return string(j)
}

// Map 转换为map[string]interface{}
func (j JSON) Map() (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(j), &m); err != nil {
		return nil, err
	}
	return m, nil
}

// JSONMap 自定义JSON类型，用于存储 map[string]string
type JSONMap map[string]string

// Value 实现driver.Valuer接口
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	data, err := json.Marshal(j)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

// Scan 实现sql.Scanner接口
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("failed to scan JSONMap value")
	}

	if len(bytes) == 0 {
		*j = nil
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// MarshalJSON 实现json.Marshaler接口
func (j JSONMap) MarshalJSON() ([]byte, error) {
	if j == nil {
		return []byte("null"), nil
	}
	return json.Marshal(map[string]string(j))
}

// UnmarshalJSON 实现json.Unmarshaler接口
func (j *JSONMap) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*j = nil
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*j = JSONMap(m)
	return nil
}
