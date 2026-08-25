// 作者：timmylau1
// 邮箱：timmyliulove2@gmail.com

package service

import (
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	githubReleasesURL = "https://api.github.com/repos/lovetimmy1314/timmypanel/releases?per_page=5"
	githubCommitURL   = "https://api.github.com/repos/lovetimmy1314/timmypanel/commits/main"

	updateBodyLimit = 64 << 10
	updateTTL       = time.Hour
	updateFailTTL   = 5 * time.Minute
)

const (
	StatusUpToDate            = "upToDate"
	StatusUpdateAvailable     = "updateAvailable"
	StatusDev                 = "dev"
	StatusUpstreamUnavailable = "upstreamUnavailable"

	ChannelDev     = "dev"
	ChannelRolling = "rolling"
	ChannelRelease = "release"
)

// CheckResult 是一次检测更新的结果。文案由前端 i18n，这里只给状态码。
type CheckResult struct {
	Current    string `json:"current"`
	Latest     string `json:"latest"`
	HasUpdate  bool   `json:"hasUpdate"`
	Channel    string `json:"channel"`
	Status     string `json:"status"`
	ReleaseURL string `json:"releaseUrl,omitempty"`
	CheckedAt  int64  `json:"checkedAt"`
	InDocker   bool   `json:"inDocker"`
}

type ghRelease struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

type ghCommit struct {
	SHA string `json:"sha"`
}

type updateCache struct {
	result CheckResult
	until  time.Time
}

type jsonGetter func(rawURL string, headers map[string]string, v any) error

// UpdateService 替前端去问 GitHub，并把结果缓存住。
//
// 浏览器不能直连：CSP 的 connect-src 只有 'self'。出站必须走 Fetcher，
// 别另起 http.Client。GitHub 未认证限额 60 次/小时/IP，所以要缓存和
// 单飞，点按钮不能变成出站放大器（决策 037）。
type UpdateService struct {
	fetcher  *Fetcher
	version  string
	now      func() time.Time
	inDocker func() bool
	getJSON  jsonGetter

	mu       sync.Mutex
	cache    *updateCache
	fetching bool
	wait     chan struct{}
}

// NewUpdateService 构造更新检测。fetcher 必须是全局那一个。
func NewUpdateService(f *Fetcher, version string) *UpdateService {
	if version == "" {
		version = "dev"
	}
	u := &UpdateService{
		fetcher:  f,
		version:  version,
		now:      time.Now,
		inDocker: detectDocker,
	}
	u.getJSON = u.fetchJSON
	return u
}

func (u *UpdateService) fetchJSON(rawURL string, headers map[string]string, v any) error {
	return u.fetcher.GetJSONHeader(rawURL, updateBodyLimit, headers, v)
}

func detectDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

// Check 返回当前构建相对上游的状态。一小时内的成功结果直接吃缓存；
// 失败缓存更短，避免一次 GitHub 抖动让按钮失效一整小时。
func (u *UpdateService) Check() (out CheckResult) {
	now := u.now()
	u.mu.Lock()
	if u.cache != nil && now.Before(u.cache.until) {
		out = u.cache.result
		u.mu.Unlock()
		return out
	}
	if u.fetching {
		ch := u.wait
		u.mu.Unlock()
		<-ch
		u.mu.Lock()
		out = CheckResult{Current: u.version, Channel: classifyVersion(u.version), Status: StatusUpstreamUnavailable, CheckedAt: now.Unix()}
		if u.cache != nil {
			out = u.cache.result
		}
		u.mu.Unlock()
		return out
	}
	u.fetching = true
	u.wait = make(chan struct{})
	u.mu.Unlock()

	// fetch 若 panic，fetching 会永远为 true，后续请求卡在 wait 上。
	// gin 的 Recovery 接得住这次请求，接不住那把锁。
	defer func() {
		rec := recover()
		if rec == nil {
			return
		}
		slog.Error("检测更新过程异常", "err", rec)
		out = CheckResult{
			Current:   u.version,
			Channel:   classifyVersion(u.version),
			Status:    StatusUpstreamUnavailable,
			CheckedAt: u.now().Unix(),
		}
		u.store(out, updateFailTTL)
	}()

	out = u.fetch()
	ttl := updateTTL
	if out.Status == StatusUpstreamUnavailable {
		ttl = updateFailTTL
	}
	u.store(out, ttl)
	return out
}

func (u *UpdateService) store(result CheckResult, ttl time.Duration) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.fetching {
		return
	}
	u.cache = &updateCache{result: result, until: u.now().Add(ttl)}
	u.fetching = false
	close(u.wait)
}

func (u *UpdateService) fetch() CheckResult {
	now := u.now()
	inDocker := false
	if u.inDocker != nil {
		inDocker = u.inDocker()
	}
	headers := map[string]string{
		"User-Agent":           "timmypanel/" + u.version,
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
	}

	var releases []ghRelease
	relErr := u.getJSON(githubReleasesURL, headers, &releases)
	var commit ghCommit
	comErr := u.getJSON(githubCommitURL, headers, &commit)
	if relErr != nil && comErr != nil {
		return CheckResult{
			Current:   u.version,
			Channel:   classifyVersion(u.version),
			Status:    StatusUpstreamUnavailable,
			CheckedAt: now.Unix(),
			InDocker:  inDocker,
		}
	}
	latest, url := pickRelease(releases)
	return decide(u.version, latest, url, commit.SHA, now, inDocker)
}

func pickRelease(releases []ghRelease) (tag, url string) {
	for _, r := range releases {
		if r.Draft || r.Prerelease {
			continue
		}
		t := strings.TrimPrefix(strings.TrimSpace(r.TagName), "v")
		if t == "" {
			continue
		}
		return t, r.HTMLURL
	}
	return "", ""
}

var (
	reSemver  = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)$`)
	reRolling = regexp.MustCompile(`^(\d{4}\.\d{2}\.\d{2})-([0-9a-fA-F]{7,})$`)
	reDate    = regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}$`)
)

func classifyVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "dev" {
		return ChannelDev
	}
	// YYYY.MM.DD 也匹配 \d+.\d+.\d+，必须先认 rolling，否则 2026.08.22
	// 会被当成 semver 2026.8.22，正式版比较全乱。
	if reRolling.MatchString(v) || reDate.MatchString(v) {
		return ChannelRolling
	}
	if reSemver.MatchString(v) {
		return ChannelRelease
	}
	return ChannelDev
}

func parseSemver(v string) (major, minor, patch int, ok bool) {
	m := reSemver.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return 0, 0, 0, false
	}
	major, _ = strconv.Atoi(m[1])
	minor, _ = strconv.Atoi(m[2])
	patch, _ = strconv.Atoi(m[3])
	return major, minor, patch, true
}

func compareSemver(a, b string) int {
	am, ai, ap, aok := parseSemver(a)
	bm, bi, bp, bok := parseSemver(b)
	if !aok || !bok {
		return 0
	}
	if am != bm {
		return am - bm
	}
	if ai != bi {
		return ai - bi
	}
	return ap - bp
}

func rollingSHA(v string) string {
	m := reRolling.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil {
		return ""
	}
	sha := strings.ToLower(m[2])
	if len(sha) > 7 {
		sha = sha[:7]
	}
	return sha
}

func shortSHA(sha string) string {
	sha = strings.ToLower(strings.TrimSpace(sha))
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// decide 是纯函数：当前版本 + 上游两份快照 → 给前端的状态。单测罩这里。
func decide(current, latestRelease, releaseURL, headSHA string, now time.Time, inDocker bool) CheckResult {
	out := CheckResult{
		Current:    current,
		Channel:    classifyVersion(current),
		CheckedAt:  now.Unix(),
		InDocker:   inDocker,
		ReleaseURL: releaseURL,
	}
	head := shortSHA(headSHA)

	switch out.Channel {
	case ChannelDev:
		out.Status = StatusDev
		out.Latest = latestRelease
		if out.Latest == "" {
			out.Latest = head
		}
		return out

	case ChannelRelease:
		out.Latest = latestRelease
		if latestRelease == "" {
			out.Status = StatusUpToDate
			return out
		}
		if compareSemver(current, latestRelease) < 0 {
			out.HasUpdate = true
			out.Status = StatusUpdateAvailable
			return out
		}
		out.Status = StatusUpToDate
		return out

	default:
		curSHA := rollingSHA(current)
		if head != "" {
			out.Latest = head
			if curSHA != "" {
				if curSHA != head {
					out.HasUpdate = true
					out.Status = StatusUpdateAvailable
					return out
				}
				out.Status = StatusUpToDate
				return out
			}
			// 本地 build.ps1 默认只打日期、没有短 sha，对不上就当有更新。
			out.HasUpdate = true
			out.Status = StatusUpdateAvailable
			return out
		}
		if latestRelease != "" {
			out.Latest = latestRelease
			out.HasUpdate = true
			out.Status = StatusUpdateAvailable
			return out
		}
		if out.Latest == "" {
			out.Latest = current
		}
		out.Status = StatusUpToDate
		return out
	}
}
