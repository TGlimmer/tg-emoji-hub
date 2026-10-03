package entity

import "testing"

func TestNormalizeSource(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"featured", "featured"},
		{"manual", "manual"},
		{"", "featured"},
		{"garbage", "featured"},
		{"FEATURED", "featured"},   // 大写不匹配，回退
		{" featured ", "featured"}, // 含空格不匹配，回退
	}
	for _, tt := range tests {
		got := NormalizeSource(tt.in)
		if got != tt.want {
			t.Errorf("NormalizeSource(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestIsValidSource(t *testing.T) {
	if !IsValidSource("featured") {
		t.Error("featured 应合法")
	}
	if !IsValidSource("manual") {
		t.Error("manual 应合法")
	}
	if IsValidSource("garbage") {
		t.Error("garbage 不应合法")
	}
}
