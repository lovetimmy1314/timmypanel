package api

import (
	"testing"

	"timmypanel/internal/model"
)

func TestShouldReplaceIcon(t *testing.T) {
	cases := []struct {
		name      string
		site      model.Site
		overwrite bool
		want      bool
	}{
		{"没有图标就补", model.Site{IconType: model.IconTypeURL, IconValue: ""}, false, true},
		{"只有空白也算没有", model.Site{IconType: model.IconTypeURL, IconValue: "   "}, false, true},
		{"图标类型是文字但值为空，照样补", model.Site{IconType: model.IconTypeText, IconValue: ""}, false, true},
		{"已有图片图标，不覆盖时不动", model.Site{IconType: model.IconTypeURL, IconValue: "/uploads/1/icons/a.png"}, false, false},
		{"已有图片图标，勾了覆盖就换", model.Site{IconType: model.IconTypeURL, IconValue: "/uploads/1/icons/a.png"}, true, true},
		// 手动选的图标库/文字图标是明确的选择，覆盖开关也不该顶掉它。
		{"图标库图标，勾了覆盖也不动", model.Site{IconType: model.IconTypeIconify, IconValue: "mdi:github"}, true, false},
		{"文字图标，勾了覆盖也不动", model.Site{IconType: model.IconTypeText, IconValue: "G"}, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldReplaceIcon(&c.site, c.overwrite); got != c.want {
				t.Errorf("shouldReplaceIcon(%+v, %v) = %v, 期望 %v", c.site, c.overwrite, got, c.want)
			}
		})
	}
}

func TestBackfillFieldsEmpty(t *testing.T) {
	if !(backfillFields{}).empty() {
		t.Error("三项全 false 应判为空")
	}
	for _, f := range []backfillFields{{Icon: true}, {Title: true}, {Description: true}} {
		if f.empty() {
			t.Errorf("%+v 不该判为空", f)
		}
	}
}
