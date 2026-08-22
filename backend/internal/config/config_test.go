// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package config

import "testing"

// envBool 的错误分支是安全相关的：原来 `c.Secure, _ = strconv.ParseBool(v)`
// 把错误丢掉，TP_SECURE=yes 会静默变成 false，会话 Cookie 就没了 Secure 标记。
func TestEnvBool(t *testing.T) {
	const key = "TP_TEST_BOOL"

	// 没设时沿用当前值，两个方向都要成立
	if got := envBool(key, true); got != true {
		t.Errorf("未设置时应沿用 true，实际 %v", got)
	}
	if got := envBool(key, false); got != false {
		t.Errorf("未设置时应沿用 false，实际 %v", got)
	}

	// ParseBool 认得的写法照常生效，带空白也要认
	for _, v := range []string{"true", "TRUE", "True", "1", "t", " true "} {
		t.Setenv(key, v)
		if got := envBool(key, false); got != true {
			t.Errorf("envBool(%q) = %v，期望 true", v, got)
		}
	}
	for _, v := range []string{"false", "FALSE", "0", "f"} {
		t.Setenv(key, v)
		if got := envBool(key, true); got != false {
			t.Errorf("envBool(%q) = %v，期望 false", v, got)
		}
	}

	// 认不出来的一律沿用原值，**不能**退成 false
	for _, v := range []string{"yes", "on", "是", "no", "off", "truthy"} {
		t.Setenv(key, v)
		if got := envBool(key, true); got != true {
			t.Errorf("envBool(%q) 把 true 改成了 %v，安全开关被静默关掉", v, got)
		}
		if got := envBool(key, false); got != false {
			t.Errorf("envBool(%q) 把 false 改成了 %v", v, got)
		}
	}
}
