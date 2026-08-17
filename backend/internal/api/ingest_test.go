package api

import (
	"strings"
	"testing"
)

// 令牌的安全前提是「原文只存在于书签里，库里只有哈希」，
// 所以这里盯住两件事：生成的形状、哈希不可逆查。
func TestNewIngestToken(t *testing.T) {
	a, err := newIngestToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := newIngestToken()
	if !strings.HasPrefix(a, ingestTokenPrefix) {
		t.Errorf("令牌缺少 %q 前缀: %q", ingestTokenPrefix, a)
	}
	// 32 字节 hex = 64 位，再加前缀。
	if len(a) != len(ingestTokenPrefix)+64 {
		t.Errorf("令牌长度 = %d，期望 %d", len(a), len(ingestTokenPrefix)+64)
	}
	if a == b {
		t.Error("两次生成的令牌相同")
	}
	if strings.Contains(hashIngestToken(a), a) {
		t.Error("哈希里不该出现令牌原文")
	}
}

// 队列是「浏览器逐个补」的核心：书签提交一个划掉一个，
// 匹配必须容忍 location.href 和卡片 URL 的协议/www/末尾斜杠差异。
func TestIngestQueuePopCanonical(t *testing.T) {
	var q ingestQueues
	q.set(1, []string{
		"https://chatgpt.com/",
		"https://www.example.com/a",
	})

	// 书签提交的是 http://chatgpt.com（无末尾斜杠），也得划得掉 https://chatgpt.com/。
	next, remaining := q.popCanonical(1, canonicalURL("http://chatgpt.com"))
	if remaining != 1 {
		t.Fatalf("remaining = %d，期望 1", remaining)
	}
	if next != "https://www.example.com/a" {
		t.Errorf("next = %q，期望队列里剩下的那一个", next)
	}

	// 划掉最后一个后队列清空，用户之间互不影响。
	q.set(2, []string{"https://other.com"})
	if next, remaining := q.popCanonical(1, canonicalURL("https://example.com/a")); next != "" || remaining != 0 {
		t.Errorf("队列清空后 next=%q remaining=%d，期望都为空", next, remaining)
	}
	if got := q.list(2); len(got) != 1 {
		t.Errorf("用户 2 的队列被动了: %v", got)
	}
}

// set 空切片是「清空队列」的语义，前端停止逐个补时靠它。
func TestIngestQueueSetEmptyClears(t *testing.T) {
	var q ingestQueues
	q.set(1, []string{"https://a.com"})
	q.set(1, nil)
	if got := q.list(1); len(got) != 0 {
		t.Errorf("set 空之后队列应为空，实际 %v", got)
	}
}
