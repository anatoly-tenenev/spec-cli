package querypayload

import (
	"bytes"
	"encoding/json"
)

type RootField struct {
	EntityType string
	Items      []map[string]any
	TotalCount int
	PageInfo   PageInfo
}

type PageInfo struct {
	Mode          string
	Limit         int
	Offset        int
	Returned      int
	HasMore       bool
	NextOffset    any
	EffectiveSort []string
}

type orderedData []RootField

type rootListPayload struct {
	Items      []map[string]any `json:"items"`
	TotalCount int              `json:"totalCount"`
	PageInfo   pageInfoPayload  `json:"pageInfo"`
}

type pageInfoPayload struct {
	Mode          string   `json:"mode"`
	Limit         int      `json:"limit"`
	Offset        int      `json:"offset"`
	Returned      int      `json:"returned"`
	HasMore       bool     `json:"hasMore"`
	NextOffset    any      `json:"nextOffset"`
	EffectiveSort []string `json:"effectiveSort"`
}

func BuildSuccess(resultState string, schema map[string]any, roots []RootField) map[string]any {
	return map[string]any{
		"result_state": resultState,
		"schema":       schema,
		"data":         orderedData(append([]RootField(nil), roots...)),
	}
}

func (data orderedData) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for idx, root := range data {
		if idx > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(root.EntityType)
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(rootListPayload{
			Items:      root.Items,
			TotalCount: root.TotalCount,
			PageInfo: pageInfoPayload{
				Mode:          root.PageInfo.Mode,
				Limit:         root.PageInfo.Limit,
				Offset:        root.PageInfo.Offset,
				Returned:      root.PageInfo.Returned,
				HasMore:       root.PageInfo.HasMore,
				NextOffset:    root.PageInfo.NextOffset,
				EffectiveSort: root.PageInfo.EffectiveSort,
			},
		})
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(value)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}
