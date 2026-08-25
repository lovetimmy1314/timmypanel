// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package service

import (
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClassifyVersion(t *testing.T) {
	cases := map[string]string{
		"":                   ChannelDev,
		"dev":                ChannelDev,
		"  dev ":             ChannelDev,
		"1.2.3":              ChannelRelease,
		"v1.2.3":             ChannelRelease,
		"2026.08.22-5ae763e": ChannelRolling,
		"2026.08.22":         ChannelRolling,
		"1.2":                ChannelDev,
		"latest":             ChannelDev,
		"2026.08.22-5AE763E": ChannelRolling,
	}
	for in, want := range cases {
		if got := classifyVersion(in); got != want {
			t.Errorf("classifyVersion(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestCompareSemver(t *testing.T) {
	if compareSemver("1.2.3", "v1.2.3") != 0 {
		t.Fatal("带不带 v 应视为同一版本")
	}
	if compareSemver("1.0.0", "1.0.1") >= 0 {
		t.Fatal("1.0.0 应小于 1.0.1")
	}
	if compareSemver("1.1.0", "1.0.9") <= 0 {
		t.Fatal("1.1.0 应大于 1.0.9")
	}
	if compareSemver("2.0.0", "1.9.9") <= 0 {
		t.Fatal("2.0.0 应大于 1.9.9")
	}
}

func TestPickReleaseSkipsDraftAndPrerelease(t *testing.T) {
	tag, url := pickRelease([]ghRelease{
		{TagName: "v1.0.1", Draft: true, HTMLURL: "https://example/draft"},
		{TagName: "v1.0.2", Prerelease: true, HTMLURL: "https://example/pre"},
		{TagName: "v1.0.3", HTMLURL: "https://example/rel"},
	})
	if tag != "1.0.3" || url != "https://example/rel" {
		t.Fatalf("应跳过 draft/prerelease，得到 tag=%q url=%q", tag, url)
	}
	if tag, url = pickRelease(nil); tag != "" || url != "" {
		t.Fatalf("空列表应得到空，得到 %q %q", tag, url)
	}
}

func TestDecide(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	const relURL = "https://github.com/lovetimmy1314/timmypanel/releases/tag/v1.0.1"

	t.Run("dev 不判断新旧", func(t *testing.T) {
		r := decide("dev", "1.0.1", relURL, "abc1234567", now, true)
		if r.Status != StatusDev || r.HasUpdate || r.Channel != ChannelDev {
			t.Fatalf("%+v", r)
		}
		if r.Latest != "1.0.1" || r.ReleaseURL != relURL || !r.InDocker {
			t.Fatalf("latest/url/docker 不对: %+v", r)
		}
	})

	t.Run("正式版落后", func(t *testing.T) {
		r := decide("1.0.0", "1.0.1", relURL, "deadbeef", now, false)
		if !r.HasUpdate || r.Status != StatusUpdateAvailable || r.Latest != "1.0.1" {
			t.Fatalf("%+v", r)
		}
	})

	t.Run("正式版已最新", func(t *testing.T) {
		r := decide("1.0.1", "1.0.1", relURL, "deadbeef", now, true)
		if r.HasUpdate || r.Status != StatusUpToDate {
			t.Fatalf("%+v", r)
		}
	})

	t.Run("正式版比 latest 还新，不打扰", func(t *testing.T) {
		r := decide("1.1.0", "1.0.9", relURL, "deadbeef", now, true)
		if r.HasUpdate || r.Status != StatusUpToDate {
			t.Fatalf("%+v", r)
		}
	})

	t.Run("正式版不看 main 的新提交", func(t *testing.T) {
		r := decide("1.0.0", "1.0.0", relURL, "ffffffff", now, true)
		if r.HasUpdate {
			t.Fatal("钉死 semver 的人不该被 rolling 提交打扰")
		}
	})

	t.Run("正式版尚无 Release", func(t *testing.T) {
		r := decide("1.0.0", "", "", "abc1234", now, true)
		if r.HasUpdate || r.Status != StatusUpToDate {
			t.Fatalf("%+v", r)
		}
	})

	t.Run("rolling 短 sha 落后", func(t *testing.T) {
		r := decide("2026.08.22-5ae763e", "", "", "abc1234ffff", now, true)
		if !r.HasUpdate || r.Status != StatusUpdateAvailable || r.Latest != "abc1234" {
			t.Fatalf("%+v", r)
		}
	})

	t.Run("rolling 已是 HEAD", func(t *testing.T) {
		r := decide("2026.08.22-5ae763e", "", "", "5ae763effffff", now, true)
		if r.HasUpdate || r.Status != StatusUpToDate || r.Latest != "5ae763e" {
			t.Fatalf("%+v", r)
		}
	})

	t.Run("日期号对上有 Release 就算有更新", func(t *testing.T) {
		r := decide("2026.08.22", "1.0.0", relURL, "", now, true)
		if !r.HasUpdate || r.Latest != "1.0.0" {
			t.Fatalf("%+v", r)
		}
	})

	t.Run("只有日期没有 sha 时跟 HEAD 比对算有更新", func(t *testing.T) {
		r := decide("2026.08.22", "", "", "abc1234ffff", now, true)
		if !r.HasUpdate || r.Latest != "abc1234" {
			t.Fatalf("%+v", r)
		}
	})
}

func TestUpdateServiceCachesSuccess(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	var calls atomic.Int32
	u := NewUpdateService(nil, "1.0.0")
	u.now = func() time.Time { return now }
	u.inDocker = func() bool { return true }
	u.getJSON = func(rawURL string, _ map[string]string, v any) error {
		calls.Add(1)
		if strings.Contains(rawURL, "/releases") {
			raw, _ := json.Marshal([]ghRelease{{TagName: "v1.0.1", HTMLURL: "https://example/r"}})
			return json.Unmarshal(raw, v)
		}
		raw, _ := json.Marshal(ghCommit{SHA: "abc1234ffff"})
		return json.Unmarshal(raw, v)
	}

	first := u.Check()
	if !first.HasUpdate || first.Latest != "1.0.1" || !first.InDocker {
		t.Fatalf("第一次 %+v", first)
	}
	second := u.Check()
	if calls.Load() != 2 {
		t.Fatalf("命中缓存后不应再出站，实际调用 %d（releases+commit=2）", calls.Load())
	}
	if second.CheckedAt != first.CheckedAt {
		t.Fatal("缓存应原样返回")
	}

	now = now.Add(updateTTL + time.Second)
	_ = u.Check()
	if calls.Load() != 4 {
		t.Fatalf("过期后应再出站，调用 %d", calls.Load())
	}
}

func TestUpdateServiceCachesFailureShorter(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	var calls atomic.Int32
	u := NewUpdateService(nil, "1.0.0")
	u.now = func() time.Time { return now }
	u.inDocker = func() bool { return false }
	u.getJSON = func(string, map[string]string, any) error {
		calls.Add(1)
		return errors.New("boom")
	}

	r := u.Check()
	if r.Status != StatusUpstreamUnavailable || r.HasUpdate {
		t.Fatalf("%+v", r)
	}
	_ = u.Check()
	if calls.Load() != 2 {
		t.Fatalf("失败也应缓存，调用 %d（两次 URL）", calls.Load())
	}

	now = now.Add(updateFailTTL + time.Second)
	_ = u.Check()
	if calls.Load() != 4 {
		t.Fatalf("失败缓存过期后应再试，调用 %d", calls.Load())
	}
}

func TestUpdateServicePartialUpstreamStillDecides(t *testing.T) {
	u := NewUpdateService(nil, "1.0.0")
	u.now = func() time.Time { return time.Unix(1, 0) }
	u.inDocker = func() bool { return false }
	u.getJSON = func(rawURL string, _ map[string]string, v any) error {
		if strings.Contains(rawURL, "/releases") {
			return errors.New("releases down")
		}
		raw, _ := json.Marshal(ghCommit{SHA: "abc1234"})
		return json.Unmarshal(raw, v)
	}
	r := u.Check()
	if r.Status == StatusUpstreamUnavailable {
		t.Fatalf("commit 通了就不应整单失败: %+v", r)
	}
	if r.Status != StatusUpToDate {
		t.Fatalf("没有 Release 时正式版保持已最新: %+v", r)
	}
}

func TestDetectDockerDoesNotPanic(t *testing.T) {
	_ = detectDocker()
}
