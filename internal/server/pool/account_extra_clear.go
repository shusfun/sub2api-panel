package pool

import (
	"bytes"
	"encoding/json"
	"sort"
)

// 可清空的数值限额：请求里显式传 null 表示删除该键（恢复为不限），缺省表示保持原值。
var clearableExtraKeys = map[string]bool{
	"window_cost_limit":          true,
	"window_cost_sticky_reserve": true,
	"daily_quota":                true,
	"total_quota":                true,
	"max_sessions":               true,
	"rpm_limit":                  true,
	"tls_fingerprint_profile_id":  true,
}

// accountExtraFields 与 AccountExtraInput 字段相同但没有方法，用来在自定义解码里复用默认解码。
type accountExtraFields AccountExtraInput

// UnmarshalJSON 保持严格解码（拒绝未知字段），并记下显式传 null 的可清空键。
func (in *AccountExtraInput) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var fields accountExtraFields
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&fields); err != nil {
		return err
	}
	*in = AccountExtraInput(fields)
	in.cleared = nil
	for key, value := range raw {
		if clearableExtraKeys[key] && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			in.cleared = append(in.cleared, key)
		}
	}
	sort.Strings(in.cleared)
	return nil
}
