package query

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

/*
查询单元说明,前面是query形式，后面代表对应sql
各种查询条件的格式说明:
- false => field = false
- 1 => field = 1
- %123% => field like %123%
- %123 => field like %123
- 123% => filed like 123%
- (,1) => field < 1
- (1,) => filed > 1
- [,1] => filed <= 1
- [1,] => filed >= 1
- [1,2] => filed >= 1 and field <= 2
- (1,2) => file > 1 and filed < 2
- t1629771113t => filed = 2021-08-24 10:11:53
- {1,2} => filed in (1,2)
- ~1 => filed <> 1
- ~{1,2} => filed not in (1,2)
- e@is null@e => field is null
- e@is not null@e => field is not null
- filed1||field2 = 3 => filed1 = 3 or filed2 = 3
- m[123]m => MATCH(field) AGAINST('123')
*/

// plainUnit 将查询条件转换为SQL条件语句
// field: 字段名
// v: 查询值
// 返回: SQL条件语句和参数值
func plainUnit(field string, v ...string) (condition string, values []interface{}) {
	// 存储所有条件
	conditions := make([]string, 0)
	// 处理OR条件(用||分隔的字段)
	fs := strings.Split(field, "||")

	for _, f := range fs {
		// 处理多个值的情况
		if len(v) > 1 {
			unitConditions := make([]string, len(v))
			for _, vv := range v {
				c, vs := plainUnit(f, vv)
				if c != "" {
					unitConditions = append(unitConditions, c)
					values = append(values, vs...)
				}
			}
			unitCondition := strings.Join(unitConditions, " AND ")
			conditions = append(conditions, fmt.Sprintf("(%s)", unitCondition))
			continue
		}

		value := v[0]
		if value == "" {
			continue
		}

		// 处理布尔值
		if !strings.EqualFold(value, "1") && !strings.EqualFold(value, "0") {
			b, err := strconv.ParseBool(value)
			if err == nil {
				conditions = append(conditions, fmt.Sprintf("`%s` = ?", f))
				values = append(values, b)
				continue
			}
		}

		// 处理LIKE查询
		if strings.HasPrefix(value, "%") || strings.HasSuffix(value, "%") {
			conditions = append(conditions, fmt.Sprintf("`%s` LIKE ?", f))
			values = append(values, value)
			continue
		}

		// 处理范围查询 (1,2)
		if strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")") {
			rang := strings.Split(value[1:len(value)-1], ",")
			if len(rang) != 2 {
				continue
			}
			if rang[0] == "" && rang[1] != "" {
				conditions = append(conditions, fmt.Sprintf("`%s` < ?", f))
				values = append(values, wrapValueType(rang[1]))
			}
			if rang[0] != "" && rang[1] == "" {
				conditions = append(conditions, fmt.Sprintf("`%s` > ?", f))
				values = append(values, wrapValueType(rang[0]))
			}
			if rang[0] != "" && rang[1] != "" {
				conditions = append(conditions, fmt.Sprintf("`%s` > ? AND `%s` < ?", f, f))
				values = append(values, wrapValueType(rang[0]), wrapValueType(rang[1]))
			}
			continue
		}

		// 处理范围查询 [1,2]
		if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			rang := strings.Split(value[1:len(value)-1], ",")
			if len(rang) != 2 {
				continue
			}
			if rang[0] == "" && rang[1] != "" {
				conditions = append(conditions, fmt.Sprintf("`%s` <= ?", f))
				values = append(values, wrapValueType(rang[1]))
			}
			if rang[0] != "" && rang[1] == "" {
				conditions = append(conditions, fmt.Sprintf("`%s` >= ?", f))
				values = append(values, wrapValueType(rang[0]))
			}
			if rang[0] != "" && rang[1] != "" {
				conditions = append(conditions, fmt.Sprintf("`%s` >= ? AND `%s` <= ?", f, f))
				values = append(values, wrapValueType(rang[0]), wrapValueType(rang[1]))
			}
			continue
		}

		// 处理IN查询 {1,2}
		if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
			conditions = append(conditions, fmt.Sprintf("`%s` in (?)", f))
			vs := strings.Split(value[1:len(value)-1], ",")
			values = append(values, vs)
			continue
		}

		// 处理不等于查询 ~1 或 NOT IN ~{1,2}
		if strings.HasPrefix(value, "~") {
			if strings.HasPrefix(value, "~{") && strings.HasSuffix(value, "}") {
				conditions = append(conditions, fmt.Sprintf("`%s` not in (?)", f))
				values = append(values, strings.Split(value[2:len(value)-1], ","))
			} else {
				conditions = append(conditions, fmt.Sprintf("`%s` <> ?", f))
				values = append(values, value[1:])
			}
			continue
		}

		// 处理IS NULL
		if value == "e@is null@e" {
			conditions = append(conditions, fmt.Sprintf("`%s` is null", f))
			continue
		}

		// 处理IS NOT NULL
		if value == "e@is not null@e" {
			conditions = append(conditions, fmt.Sprintf("`%s` is not null", f))
			continue
		}

		// 处理全文搜索
		if strings.HasPrefix(value, "m[") && strings.HasSuffix(value, "]m") {
			conditions = append(conditions, fmt.Sprintf("MATCH(`%s`) AGAINST(?)", f))
			values = append(values, value[2:len(value)-2])
			continue
		}

		// 默认等于查询
		conditions = append(conditions, fmt.Sprintf("`%s` = ?", f))
		values = append(values, wrapValueType(value))
	}

	// 组合所有条件
	condition = strings.Join(conditions, " OR ")
	if len(conditions) > 1 {
		condition = fmt.Sprintf("(%s)", condition)
	}
	return
}

// wrapValueType 处理特殊值类型转换
// 主要处理时间戳格式 t1629771113t
func wrapValueType(v string) interface{} {
	if strings.HasPrefix(v, "t") && strings.HasSuffix(v, "t") {
		t, err := cast.ToInt64E(v[1 : len(v)-1])
		if err != nil {
			return v
		}
		return time.Unix(t, 0).Format(time.RFC3339)
	}
	return v
}

// Query 封装url.Values用于处理查询参数
type Query struct {
	url.Values
}

// Plains 将查询参数转换为SQL条件
// fields: 允许查询的字段列表
// 返回: SQL条件和参数值
// Plains 最后返回一个where查询条件
func (q Query) Plains(fields ...string) []interface{} {
	var conditions []string
	var values []interface{}
	for field, v := range q.Values {
		if !includeOr(field) {
			if !inSlice(field, fields) {
				continue
			}
		}
		c, vs := plainUnit(field, v...)
		if c != "" {
			conditions = append(conditions, c)
			values = append(values, vs...)
		}
	}
	condition := strings.Join(conditions, " AND ")
	return append([]interface{}{condition}, values...)
}

// inSlice 检查字段是否在查询列表中
func inSlice(field string, fields []string) bool {
	for _, f := range fields {
		if f == field {
			return true
		}
	}
	return false
}

// includeOr 检查字段是否包含OR条件
func includeOr(field string) bool {
	return strings.Contains(field, "||")
}

const (
	fieldOffset   = "offset"
	fieldLimit    = "limit"
	fieldOrder    = "order_by"
	fieldPreload  = "populate"
	defaultLimit  = 20
	defaultOffset = 0
	maxLimit      = 100
)

type QueryParams struct {
	Query       Query                    `json:"query"`
	Limit       int                      `json:"limit"`
	Offset      int                      `json:"offset"`
	Order       string                   `json:"order"`
	Group       string                   `json:"group"`
	Having      string                   `json:"having"`
	Joins       string                   `json:"joins"`
	CustomQuery map[string][]interface{} `json:"custom_query"`
	Select      string                   `json:"select"`
	TableName   string                   `json:"table_name"`
	Preload     []string                 `json:"preload"`
}

// NewQueryParams 创建一个QueryParams实例
func NewQueryParams(c *gin.Context) *QueryParams {
	// 获取查询参数
	values := c.Request.URL.Query()
	params := &QueryParams{
		Limit:  defaultLimit,
		Offset: defaultOffset,
	}
	// 获取limit
	limit := cast.ToInt(c.Query(fieldLimit))
	if limit > 0 {
		if limit > maxLimit {
			params.Limit = maxLimit
		} else {
			params.Limit = limit
		}
	}
	values.Del(fieldLimit)

	// 获取offset
	offset := cast.ToInt(c.Query(fieldOffset))
	if offset > 0 {
		params.Offset = offset
	}
	values.Del(fieldOffset)

	// 获取order
	orders := c.QueryArray(fieldOrder)
	sortBys := make([]string, 0)
	for _, order := range orders {
		// 如果order以-开头，则表示降序
		if strings.HasPrefix(order, "-") {
			order = strings.TrimLeft(order, "-")
			sortBys = append(sortBys, fmt.Sprintf("%s %s", order, "desc"))
		} else {
			sortBys = append(sortBys, fmt.Sprintf("%s %s", order, "asc"))
		}
	}
	params.Order = strings.Join(sortBys, ",")

	values.Del(fieldOrder)

	// 获取preload
	preload := cast.ToString(c.Query(fieldPreload))
	if len(preload) > 0 {
		preload = strings.Trim(preload, "[]")
		preloads := strings.Split(preload, ",")
		// 将preload转换为大写
		for _, p := range preloads {
			p = strings.Trim(p, "\"")
			params.Preload = append(params.Preload, strings.Title(p))
		}
		values.Del(fieldPreload)
	}

	// 创建QueryParams实例
	params.Query = Query{Values: values}
	params.CustomQuery = make(map[string][]interface{})

	return params
}

// NewCustomQueryParams 创建一个CustomQueryParams实例
func NewCustomQueryParams() *QueryParams {
	qp := &QueryParams{
		Query:       Query{Values: make(url.Values)},
		Limit:       defaultLimit,
		CustomQuery: map[string][]interface{}{},
	}
	return qp
}

// Add 添加查询参数
func (q *QueryParams) Add(key, value string) {
	q.Query.Add(key, value)
}

// Del 删除查询参数
func (q *QueryParams) Del(key string) {
	q.Query.Del(key)
}

// Get 获取查询参数
func (q *QueryParams) Get(key string) string {
	return q.Query.Get(key)
}

// Set 设置查询参数
func (q *QueryParams) Set(key string, values []string) {
	q.Query.Values[key] = values
}

// Replace 替换查询参数
func (q *QueryParams) Replace(oldKey, newKey string) {
	value, ok := q.Query.Values[oldKey]
	if ok {
		q.Set(newKey, value)
		q.Del(oldKey)
	}
}

// ReplaceHas2Compare 把bool转为对数量的>/=判断
func (q *QueryParams) ReplaceHas2Compare(oldKey, newKey string) {
	if has := q.Get(oldKey); len(has) > 0 {
		if cast.ToBool(has) {
			q.Set(newKey, []string{"[0,]"})
		} else {
			q.Set(newKey, []string{"0"})
		}
		q.Del(oldKey)
	}
}

// AddCustomQuery 添加自定义查询
func (q *QueryParams) AddCustomQuery(queryStr string, value ...interface{}) {
	q.CustomQuery[queryStr] = value
}

// DelCustomQuery 删除自定义查询
func (q *QueryParams) DelCustomQuery(queryStr string) {
	delete(q.CustomQuery, queryStr)
}

// AddSelect添加select
func (q *QueryParams) AddSelect(selectStr string) {
	q.Select = selectStr
}
