package pool

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func decodeStrict(t *testing.T, body string, out any) error {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader([]byte(body)))
	dec.DisallowUnknownFields()
	return dec.Decode(out)
}

func TestAccountExtraNullClearsLimit(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		current map[string]any
		want    map[string]any
	}{
		{
			name:    "null 删除，缺省保留，数值覆盖",
			body:    `{"extra":{"rpm_limit":null,"max_sessions":3}}`,
			current: map[string]any{"rpm_limit": 10, "max_sessions": 1, "daily_quota": 5.0, "private": "keep"},
			want:    map[string]any{"max_sessions": 3, "daily_quota": 5.0, "private": "keep"},
		},
		{
			name:    "不可清空的键传 null 不删除",
			body:    `{"extra":{"codex_cli_only":null}}`,
			current: map[string]any{"codex_cli_only": true},
			want:    map[string]any{"codex_cli_only": true},
		},
		{
			name:    "四个限额同时清空",
			body:    `{"extra":{"window_cost_limit":null,"daily_quota":null,"rpm_limit":null,"max_sessions":null}}`,
			current: map[string]any{"window_cost_limit": 1.0, "daily_quota": 2.0, "rpm_limit": 3, "max_sessions": 4},
			want:    map[string]any{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var input accountWriteInput
			if err := decodeStrict(t, tc.body, &input); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if input.Extra == nil {
				t.Fatal("extra not decoded")
			}
			input.Extra.applyTo(tc.current)
			if !reflect.DeepEqual(tc.current, tc.want) {
				t.Fatalf("merged extra = %#v, want %#v", tc.current, tc.want)
			}
		})
	}
}

func TestAccountExtraStillRejectsUnknownFields(t *testing.T) {
	var input accountWriteInput
	if err := decodeStrict(t, `{"extra":{"bogus":1}}`, &input); err == nil {
		t.Fatal("unknown extra field accepted")
	}
}

func TestAccountExtraNullObjectKeepsPointerNil(t *testing.T) {
	var input accountWriteInput
	if err := decodeStrict(t, `{"extra":null}`, &input); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if input.Extra != nil {
		t.Fatal("extra:null should leave Extra nil")
	}
}

func TestAccountExtraTLSProfileID(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		current map[string]any
		want    map[string]any
		valid   bool
	}{
		{"设置档案", `{"extra":{"tls_fingerprint_profile_id":7}}`, map[string]any{}, map[string]any{"tls_fingerprint_profile_id": int64(7)}, true},
		{"null 清除档案", `{"extra":{"tls_fingerprint_profile_id":null}}`, map[string]any{"tls_fingerprint_profile_id": 7, "private": "keep"}, map[string]any{"private": "keep"}, true},
		{"非法 ID 被拒绝", `{"extra":{"tls_fingerprint_profile_id":0}}`, map[string]any{}, map[string]any{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var input accountWriteInput
			if err := decodeStrict(t, tc.body, &input); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if input.Extra.valid() != tc.valid {
				t.Fatalf("valid = %v, want %v", input.Extra.valid(), tc.valid)
			}
			if !tc.valid {
				return
			}
			input.Extra.applyTo(tc.current)
			if !reflect.DeepEqual(tc.current, tc.want) {
				t.Fatalf("merged extra = %#v, want %#v", tc.current, tc.want)
			}
		})
	}
}
