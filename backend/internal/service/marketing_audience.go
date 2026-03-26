package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type MarketingAudienceMode string

const (
	MarketingAudienceModeAll      MarketingAudienceMode = "all"
	MarketingAudienceModeSelected MarketingAudienceMode = "selected"
	MarketingAudienceModeRules    MarketingAudienceMode = "rules"
)

type MarketingAudienceCombinator string

const (
	MarketingAudienceCombinatorAnd MarketingAudienceCombinator = "and"
	MarketingAudienceCombinatorOr  MarketingAudienceCombinator = "or"
)

type MarketingAudienceNodeType string

const (
	MarketingAudienceNodeTypeGroup     MarketingAudienceNodeType = "group"
	MarketingAudienceNodeTypeCondition MarketingAudienceNodeType = "condition"
)

type MarketingAudienceOperator string

const (
	MarketingAudienceOperatorEq          MarketingAudienceOperator = "eq"
	MarketingAudienceOperatorNeq         MarketingAudienceOperator = "neq"
	MarketingAudienceOperatorContains    MarketingAudienceOperator = "contains"
	MarketingAudienceOperatorNotContains MarketingAudienceOperator = "not_contains"
	MarketingAudienceOperatorIn          MarketingAudienceOperator = "in"
	MarketingAudienceOperatorNotIn       MarketingAudienceOperator = "not_in"
	MarketingAudienceOperatorGte         MarketingAudienceOperator = "gte"
	MarketingAudienceOperatorLte         MarketingAudienceOperator = "lte"
	MarketingAudienceOperatorIsEmpty     MarketingAudienceOperator = "is_empty"
	MarketingAudienceOperatorIsNotEmpty  MarketingAudienceOperator = "is_not_empty"
)

type MarketingAudienceNode struct {
	Type       MarketingAudienceNodeType   `json:"type"`
	Combinator MarketingAudienceCombinator `json:"combinator,omitempty"`
	Rules      []MarketingAudienceNode     `json:"rules,omitempty"`
	Field      string                      `json:"field,omitempty"`
	Operator   MarketingAudienceOperator   `json:"operator,omitempty"`
	Value      interface{}                 `json:"value,omitempty"`
}

type marketingAudienceFieldKind string

const (
	marketingAudienceFieldKindString marketingAudienceFieldKind = "string"
	marketingAudienceFieldKindBool   marketingAudienceFieldKind = "bool"
	marketingAudienceFieldKindInt    marketingAudienceFieldKind = "int"
	marketingAudienceFieldKindTime   marketingAudienceFieldKind = "time"
)

type marketingAudienceFieldDef struct {
	Column         string
	Kind           marketingAudienceFieldKind
	AllowContains  bool
	AllowRange     bool
	AllowSet       bool
	AllowEmpty     bool
	AllowEmptyList bool
}

const (
	marketingAudienceMaxDepth = 6
	marketingAudienceMaxNodes = 64
)

var marketingAudienceFieldDefs = map[string]marketingAudienceFieldDef{
	"id": {
		Column:     "id",
		Kind:       marketingAudienceFieldKindInt,
		AllowRange: true,
		AllowSet:   true,
	},
	"email": {
		Column:        "email",
		Kind:          marketingAudienceFieldKindString,
		AllowContains: true,
		AllowSet:      true,
		AllowEmpty:    true,
	},
	"name": {
		Column:        "name",
		Kind:          marketingAudienceFieldKindString,
		AllowContains: true,
		AllowSet:      true,
		AllowEmpty:    true,
	},
	"phone": {
		Column:        "phone",
		Kind:          marketingAudienceFieldKindString,
		AllowContains: true,
		AllowSet:      true,
		AllowEmpty:    true,
	},
	"is_active": {
		Column:   "is_active",
		Kind:     marketingAudienceFieldKindBool,
		AllowSet: true,
	},
	"email_verified": {
		Column:   "email_verified",
		Kind:     marketingAudienceFieldKindBool,
		AllowSet: true,
	},
	"email_notify_marketing": {
		Column:   "email_notify_marketing",
		Kind:     marketingAudienceFieldKindBool,
		AllowSet: true,
	},
	"sms_notify_marketing": {
		Column:   "sms_notify_marketing",
		Kind:     marketingAudienceFieldKindBool,
		AllowSet: true,
	},
	"locale": {
		Column:        "locale",
		Kind:          marketingAudienceFieldKindString,
		AllowContains: true,
		AllowSet:      true,
		AllowEmpty:    true,
	},
	"country": {
		Column:        "country",
		Kind:          marketingAudienceFieldKindString,
		AllowContains: true,
		AllowSet:      true,
		AllowEmpty:    true,
	},
	"total_order_count": {
		Column:     "total_order_count",
		Kind:       marketingAudienceFieldKindInt,
		AllowRange: true,
		AllowSet:   true,
	},
	"total_spent_minor": {
		Column:     "total_spent_minor",
		Kind:       marketingAudienceFieldKindInt,
		AllowRange: true,
		AllowSet:   true,
	},
	"last_login_at": {
		Column:     "last_login_at",
		Kind:       marketingAudienceFieldKindTime,
		AllowRange: true,
		AllowSet:   true,
		AllowEmpty: true,
	},
	"created_at": {
		Column:     "created_at",
		Kind:       marketingAudienceFieldKindTime,
		AllowRange: true,
		AllowSet:   true,
	},
}

func NormalizeMarketingAudienceMode(raw string) (MarketingAudienceMode, error) {
	mode := MarketingAudienceMode(strings.ToLower(strings.TrimSpace(raw)))
	if mode == "" {
		return "", nil
	}

	switch mode {
	case MarketingAudienceModeAll, MarketingAudienceModeSelected, MarketingAudienceModeRules:
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported marketing audience mode: %s", raw)
	}
}

func MarketingAudienceNodeToMap(node *MarketingAudienceNode) (map[string]interface{}, error) {
	if node == nil {
		return nil, nil
	}

	body, err := json.Marshal(node)
	if err != nil {
		return nil, err
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func ValidateMarketingAudienceQuery(node *MarketingAudienceNode) error {
	if node == nil {
		return errors.New("audience query is required")
	}

	nodeCount := 0
	return validateMarketingAudienceNode(node, 1, &nodeCount)
}

func HasMeaningfulMarketingAudienceQuery(node *MarketingAudienceNode) bool {
	if node == nil {
		return false
	}
	switch node.Type {
	case MarketingAudienceNodeTypeCondition:
		return strings.TrimSpace(node.Field) != "" && strings.TrimSpace(string(node.Operator)) != ""
	case MarketingAudienceNodeTypeGroup:
		for i := range node.Rules {
			if HasMeaningfulMarketingAudienceQuery(&node.Rules[i]) {
				return true
			}
		}
	}
	return false
}

func ApplyMarketingAudienceQuery(query *gorm.DB, node *MarketingAudienceNode) (*gorm.DB, error) {
	if query == nil {
		return nil, errors.New("query is required")
	}
	if err := ValidateMarketingAudienceQuery(node); err != nil {
		return nil, err
	}

	conditionSQL, conditionArgs, err := buildMarketingAudienceConditionExpression(node)
	if err != nil {
		return nil, err
	}
	return query.Where(conditionSQL, conditionArgs...), nil
}

func validateMarketingAudienceNode(node *MarketingAudienceNode, depth int, nodeCount *int) error {
	if node == nil {
		return errors.New("audience node is required")
	}
	if depth > marketingAudienceMaxDepth {
		return fmt.Errorf("audience query depth cannot exceed %d", marketingAudienceMaxDepth)
	}
	if nodeCount != nil {
		*nodeCount = *nodeCount + 1
		if *nodeCount > marketingAudienceMaxNodes {
			return fmt.Errorf("audience query cannot exceed %d rules", marketingAudienceMaxNodes)
		}
	}

	switch node.Type {
	case MarketingAudienceNodeTypeGroup:
		return validateMarketingAudienceGroup(node, depth, nodeCount)
	case MarketingAudienceNodeTypeCondition:
		return validateMarketingAudienceCondition(node)
	default:
		return fmt.Errorf("unsupported audience node type: %s", node.Type)
	}
}

func validateMarketingAudienceGroup(node *MarketingAudienceNode, depth int, nodeCount *int) error {
	switch node.Combinator {
	case MarketingAudienceCombinatorAnd, MarketingAudienceCombinatorOr:
	default:
		return fmt.Errorf("unsupported audience combinator: %s", node.Combinator)
	}
	if len(node.Rules) == 0 {
		return errors.New("audience group must contain at least one rule")
	}
	for i := range node.Rules {
		if err := validateMarketingAudienceNode(&node.Rules[i], depth+1, nodeCount); err != nil {
			return err
		}
	}
	return nil
}

func validateMarketingAudienceCondition(node *MarketingAudienceNode) error {
	field := strings.TrimSpace(node.Field)
	fieldDef, ok := marketingAudienceFieldDefs[field]
	if !ok {
		return fmt.Errorf("unsupported audience field: %s", node.Field)
	}

	switch node.Operator {
	case MarketingAudienceOperatorEq, MarketingAudienceOperatorNeq:
		return validateMarketingAudienceValue(fieldDef, node.Operator, node.Value)
	case MarketingAudienceOperatorContains, MarketingAudienceOperatorNotContains:
		if !fieldDef.AllowContains {
			return fmt.Errorf("operator %s is not supported for field %s", node.Operator, node.Field)
		}
		return validateMarketingAudienceValue(fieldDef, node.Operator, node.Value)
	case MarketingAudienceOperatorIn, MarketingAudienceOperatorNotIn:
		if !fieldDef.AllowSet {
			return fmt.Errorf("operator %s is not supported for field %s", node.Operator, node.Field)
		}
		return validateMarketingAudienceValue(fieldDef, node.Operator, node.Value)
	case MarketingAudienceOperatorGte, MarketingAudienceOperatorLte:
		if !fieldDef.AllowRange {
			return fmt.Errorf("operator %s is not supported for field %s", node.Operator, node.Field)
		}
		return validateMarketingAudienceValue(fieldDef, node.Operator, node.Value)
	case MarketingAudienceOperatorIsEmpty, MarketingAudienceOperatorIsNotEmpty:
		if !fieldDef.AllowEmpty {
			return fmt.Errorf("operator %s is not supported for field %s", node.Operator, node.Field)
		}
		return nil
	default:
		return fmt.Errorf("unsupported audience operator: %s", node.Operator)
	}
}

func validateMarketingAudienceValue(fieldDef marketingAudienceFieldDef, operator MarketingAudienceOperator, value interface{}) error {
	switch operator {
	case MarketingAudienceOperatorEq, MarketingAudienceOperatorNeq, MarketingAudienceOperatorContains, MarketingAudienceOperatorNotContains:
		_, err := normalizeMarketingAudienceScalarValue(fieldDef, value)
		return err
	case MarketingAudienceOperatorIn, MarketingAudienceOperatorNotIn:
		_, err := normalizeMarketingAudienceSliceValue(fieldDef, value)
		return err
	case MarketingAudienceOperatorGte, MarketingAudienceOperatorLte:
		_, err := normalizeMarketingAudienceScalarValue(fieldDef, value)
		return err
	default:
		return nil
	}
}

func buildMarketingAudienceConditionExpression(node *MarketingAudienceNode) (string, []interface{}, error) {
	switch node.Type {
	case MarketingAudienceNodeTypeGroup:
		parts := make([]string, 0, len(node.Rules))
		args := make([]interface{}, 0, len(node.Rules))
		separator := " AND "
		if node.Combinator == MarketingAudienceCombinatorOr {
			separator = " OR "
		}
		for index := range node.Rules {
			childSQL, childArgs, err := buildMarketingAudienceConditionExpression(&node.Rules[index])
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, childSQL)
			args = append(args, childArgs...)
		}
		return "(" + strings.Join(parts, separator) + ")", args, nil
	case MarketingAudienceNodeTypeCondition:
		return buildMarketingAudienceLeafExpression(node)
	default:
		return "", nil, fmt.Errorf("unsupported audience node type: %s", node.Type)
	}
}

func buildMarketingAudienceLeafExpression(node *MarketingAudienceNode) (string, []interface{}, error) {
	fieldDef := marketingAudienceFieldDefs[strings.TrimSpace(node.Field)]
	column := fieldDef.Column

	switch node.Operator {
	case MarketingAudienceOperatorEq:
		value, err := normalizeMarketingAudienceScalarValue(fieldDef, node.Value)
		if err != nil {
			return "", nil, err
		}
		if fieldDef.Kind == marketingAudienceFieldKindString {
			return "LOWER(" + column + ") = LOWER(?)", []interface{}{value}, nil
		}
		return column + " = ?", []interface{}{value}, nil
	case MarketingAudienceOperatorNeq:
		value, err := normalizeMarketingAudienceScalarValue(fieldDef, node.Value)
		if err != nil {
			return "", nil, err
		}
		if fieldDef.Kind == marketingAudienceFieldKindString {
			return "LOWER(" + column + ") <> LOWER(?)", []interface{}{value}, nil
		}
		return column + " <> ?", []interface{}{value}, nil
	case MarketingAudienceOperatorContains:
		value, err := normalizeMarketingAudienceScalarValue(fieldDef, node.Value)
		if err != nil {
			return "", nil, err
		}
		text := strings.TrimSpace(value.(string))
		return "LOWER(" + column + ") LIKE ?", []interface{}{"%" + strings.ToLower(text) + "%"}, nil
	case MarketingAudienceOperatorNotContains:
		value, err := normalizeMarketingAudienceScalarValue(fieldDef, node.Value)
		if err != nil {
			return "", nil, err
		}
		text := strings.TrimSpace(value.(string))
		return "LOWER(" + column + ") NOT LIKE ?", []interface{}{"%" + strings.ToLower(text) + "%"}, nil
	case MarketingAudienceOperatorIn:
		values, err := normalizeMarketingAudienceSliceValue(fieldDef, node.Value)
		if err != nil {
			return "", nil, err
		}
		if fieldDef.Kind == marketingAudienceFieldKindString {
			loweredValues := make([]string, 0, len(values))
			for _, value := range values {
				loweredValues = append(loweredValues, strings.ToLower(value.(string)))
			}
			return "LOWER(" + column + ") IN ?", []interface{}{loweredValues}, nil
		}
		return column + " IN ?", []interface{}{values}, nil
	case MarketingAudienceOperatorNotIn:
		values, err := normalizeMarketingAudienceSliceValue(fieldDef, node.Value)
		if err != nil {
			return "", nil, err
		}
		if fieldDef.Kind == marketingAudienceFieldKindString {
			loweredValues := make([]string, 0, len(values))
			for _, value := range values {
				loweredValues = append(loweredValues, strings.ToLower(value.(string)))
			}
			return "LOWER(" + column + ") NOT IN ?", []interface{}{loweredValues}, nil
		}
		return column + " NOT IN ?", []interface{}{values}, nil
	case MarketingAudienceOperatorGte:
		value, err := normalizeMarketingAudienceScalarValue(fieldDef, node.Value)
		if err != nil {
			return "", nil, err
		}
		return column + " >= ?", []interface{}{value}, nil
	case MarketingAudienceOperatorLte:
		value, err := normalizeMarketingAudienceScalarValue(fieldDef, node.Value)
		if err != nil {
			return "", nil, err
		}
		return column + " <= ?", []interface{}{value}, nil
	case MarketingAudienceOperatorIsEmpty:
		if fieldDef.Kind == marketingAudienceFieldKindString {
			return "(" + column + " IS NULL OR TRIM(" + column + ") = '')", nil, nil
		}
		return column + " IS NULL", nil, nil
	case MarketingAudienceOperatorIsNotEmpty:
		if fieldDef.Kind == marketingAudienceFieldKindString {
			return "(" + column + " IS NOT NULL AND TRIM(" + column + ") <> '')", nil, nil
		}
		return column + " IS NOT NULL", nil, nil
	default:
		return "", nil, fmt.Errorf("unsupported audience operator: %s", node.Operator)
	}
}

func normalizeMarketingAudienceScalarValue(fieldDef marketingAudienceFieldDef, value interface{}) (interface{}, error) {
	switch fieldDef.Kind {
	case marketingAudienceFieldKindString:
		return normalizeMarketingAudienceStringValue(value)
	case marketingAudienceFieldKindBool:
		return normalizeMarketingAudienceBoolValue(value)
	case marketingAudienceFieldKindInt:
		return normalizeMarketingAudienceIntValue(value)
	case marketingAudienceFieldKindTime:
		return normalizeMarketingAudienceTimeValue(value)
	default:
		return nil, errors.New("unsupported audience field kind")
	}
}

func normalizeMarketingAudienceSliceValue(fieldDef marketingAudienceFieldDef, value interface{}) ([]interface{}, error) {
	rawValues := make([]interface{}, 0)
	switch typed := value.(type) {
	case []interface{}:
		rawValues = append(rawValues, typed...)
	case []string:
		for _, item := range typed {
			rawValues = append(rawValues, item)
		}
	case string:
		for _, part := range strings.Split(typed, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			rawValues = append(rawValues, trimmed)
		}
	default:
		return nil, errors.New("audience value must be an array")
	}

	if len(rawValues) == 0 {
		return nil, errors.New("audience value array cannot be empty")
	}

	result := make([]interface{}, 0, len(rawValues))
	for _, rawValue := range rawValues {
		normalizedValue, err := normalizeMarketingAudienceScalarValue(fieldDef, rawValue)
		if err != nil {
			return nil, err
		}
		result = append(result, normalizedValue)
	}
	return result, nil
}

func normalizeMarketingAudienceStringValue(value interface{}) (string, error) {
	text, ok := value.(string)
	if !ok {
		return "", errors.New("audience value must be a string")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("audience value cannot be empty")
	}
	return text, nil
}

func normalizeMarketingAudienceBoolValue(value interface{}) (bool, error) {
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err != nil {
			return false, errors.New("audience value must be boolean")
		}
		return parsed, nil
	default:
		return false, errors.New("audience value must be boolean")
	}
}

func normalizeMarketingAudienceIntValue(value interface{}) (int64, error) {
	switch typed := value.(type) {
	case int:
		return int64(typed), nil
	case int32:
		return int64(typed), nil
	case int64:
		return typed, nil
	case float64:
		if typed != float64(int64(typed)) {
			return 0, errors.New("audience value must be an integer")
		}
		return int64(typed), nil
	case json.Number:
		return typed.Int64()
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err != nil {
			return 0, errors.New("audience value must be an integer")
		}
		return parsed, nil
	default:
		return 0, errors.New("audience value must be an integer")
	}
}

func normalizeMarketingAudienceTimeValue(value interface{}) (time.Time, error) {
	text, ok := value.(string)
	if !ok {
		return time.Time{}, errors.New("audience value must be a datetime string")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return time.Time{}, errors.New("audience value cannot be empty")
	}

	layoutsWithLocation := []string{
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layoutsWithLocation {
		if parsed, err := time.ParseInLocation(layout, text, time.Local); err == nil {
			return parsed, nil
		}
	}

	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, errors.New("audience value must be a valid datetime")
}
