// Package title 基于正则的文件名标题识别。
// 识别顺序：用户自定义规则（按序）→ 同名 NFO → 内置默认规则 → 文件名清洗兜底。
package title

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/marsjimmy/strmsub/internal/metadata/nfo"
)

// Rule 一条识别规则；Pattern 支持命名分组 (?P<title>...) (?P<year>...) (?P<season>...) (?P<episode>...)
type Rule struct {
	ID      int64
	Name    string
	Pattern string
}

type Result struct {
	Title    string
	Year     int
	Season   int
	Episode  int
	RuleID   int64 // >0 自定义规则；0 内置；-1 NFO；-2 清洗兜底
	RuleName string
}

type compiled struct {
	rule Rule
	re   *regexp.Regexp
}

type Recognizer struct {
	custom []compiled
}

func New(rules []Rule) *Recognizer {
	r := &Recognizer{}
	for _, rl := range rules {
		re, err := regexp.Compile(rl.Pattern)
		if err != nil {
			continue // 非法正则跳过，不中断
		}
		r.custom = append(r.custom, compiled{rl, re})
	}
	return r
}

// Recognize 识别视频文件路径的标题信息
func (r *Recognizer) Recognize(videoPath string) Result {
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	// 1. 自定义规则
	for _, c := range r.custom {
		if res, ok := applyRule(c.re, base); ok {
			res.RuleID, res.RuleName = c.rule.ID, c.rule.Name
			return res
		}
	}
	// 2. 同名 NFO
	if res, ok := fromNFO(videoPath); ok {
		return res
	}
	// 3. 内置默认规则
	for i, p := range defaultPatterns {
		if res, ok := applyRule(p.re, base); ok {
			res.RuleID, res.RuleName = 0, p.name
			_ = i
			return res
		}
	}
	// 4. 清洗兜底
	return Result{Title: cleanTitle(base), RuleID: -2, RuleName: "文件名清洗"}
}

// Test 用指定 pattern 测试文件名，返回识别结果或错误说明
func Test(pattern, filename string) (Result, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return Result{}, err
	}
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	if res, ok := applyRule(re, base); ok {
		return res, nil
	}
	return Result{}, errNoMatch
}

var errNoMatch = errNoMatchT{}

type errNoMatchT struct{}

func (errNoMatchT) Error() string { return "正则未匹配到标题" }

func applyRule(re *regexp.Regexp, base string) (Result, bool) {
	m := re.FindStringSubmatch(base)
	if m == nil {
		return Result{}, false
	}
	var res Result
	for i, name := range re.SubexpNames() {
		if i == 0 || name == "" || m[i] == "" {
			continue
		}
		switch name {
		case "title":
			res.Title = cleanTitle(m[i])
		case "year":
			res.Year = atoi(m[i])
		case "season":
			res.Season = atoi(m[i])
		case "episode":
			res.Episode = atoi(m[i])
		}
	}
	if res.Title == "" {
		return Result{}, false
	}
	return res, true
}

func fromNFO(videoPath string) (Result, bool) {
	nfoPath := strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + ".nfo"
	if _, err := os.Stat(nfoPath); err != nil {
		return Result{}, false
	}
	mi, ok := nfo.ParseFile(nfoPath)
	if !ok || mi.Title == "" {
		return Result{}, false
	}
	return Result{Title: mi.Title, Year: mi.Year, Season: mi.Season, Episode: mi.Episode,
		RuleID: -1, RuleName: "NFO"}, true
}

type defPattern struct {
	name string
	re   *regexp.Regexp
}

var defaultPatterns = []defPattern{
	// 番号：ABC-123 / SONE028（整体作为标题，供番号类字幕源直接搜索）；
	// 无分隔符时要求字母全大写，避免把 ShowE03 这类误判
	{"番号", regexp.MustCompile(`^(?P<title>(?:[A-Za-z]{2,6}[\-_]\d{2,5}|[A-Z]{2,6}\d{2,5}))$`)},
	// S01E01 / s1e2
	{"剧集 SxxExx", regexp.MustCompile(`(?i)^(?P<title>.+?)[.\s_\-]+[Ss](?P<season>\d{1,2})[Ee](?P<episode>\d{1,3})`)},
	// E01 / EP02 / 第3集（分隔符可选）
	{"剧集 Exx", regexp.MustCompile(`(?i)^(?P<title>.+?)(?:[.\s_\-]+)?(?:[Ee][Pp]?|[第])(?P<episode>\d{1,3})(?:[集话]|$)`)},
	// Title.2020. / Title (2020) / Title-2020
	{"电影 年份", regexp.MustCompile(`^(?P<title>.+?)[.\s_\-\(\[](?P<year>(?:19|20)\d{2})[\)\]\.\s_\-]`)},
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// cleanTitle 清洗标题：点/下划线转空格，去括号内多余信息，压缩空白。
// 注意保留 "-"（番号 ABC-123、Spider-Man 这类标题需要它）。
func cleanTitle(s string) string {
	s = strings.ReplaceAll(s, ".", " ")
	s = strings.ReplaceAll(s, "_", " ")
	// 去掉方括号/花括号内容（常为字幕组/压制信息），保留圆括号（常为年份）
	s = regexp.MustCompile(`\[[^\]]*\]`).ReplaceAllString(s, " ")
	s = regexp.MustCompile(`\{[^}]*\}`).ReplaceAllString(s, " ")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(s), " ")
	return s
}

// DefaultRuleHint 给设置页展示的默认规则说明
func DefaultRuleHint() []string {
	out := make([]string, len(defaultPatterns))
	for i, p := range defaultPatterns {
		out[i] = p.name + "：" + p.re.String()
	}
	return out
}
