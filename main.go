package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Nginx combined ログのパターン
var logPattern = regexp.MustCompile(
	`^(\S+)` + // IP
		` \S+` + // ident
		` \S+` + // authuser
		` \[([^\]]+)\]` + // datetime
		` "(\S+) (\S+) \S+"` + // method, path
		` (\d{3})` + // status
		` \S+` + // size
		` "[^"]*"` + // referer
		` "[^"]*"`, // user-agent
)

type record struct {
	ip     string
	hour   int
	path   string
	status string
}

func parseLog(filepath string) ([]record, error) {
	f, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var records []record
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		matches := logPattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		ip := matches[1]
		datetime := matches[2]
		path := matches[4]
		status := matches[5]

		hour := 0
		// datetime 例: 09/Nov/2026:14:23:01 +0900
		parts := strings.SplitN(datetime, ":", 2)
		if len(parts) >= 2 {
			timeParts := strings.Split(parts[1], ":")
			if len(timeParts) >= 1 {
				h, err := strconv.Atoi(timeParts[0])
				if err == nil {
					hour = h
				}
			}
		}

		records = append(records, record{
			ip:     ip,
			hour:   hour,
			path:   path,
			status: status,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

// kv はカウント結果のキーと値のペア
type kv struct {
	key   string
	count int
}

// countAndSort はマップをカウント降順でソートしたスライスを返す
func countAndSort(m map[string]int) []kv {
	result := make([]kv, 0, len(m))
	for k, v := range m {
		result = append(result, kv{k, v})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].count != result[j].count {
			return result[i].count > result[j].count
		}
		return result[i].key < result[j].key
	})
	return result
}

func printSection(title string) {
	fmt.Println()
	fmt.Printf("--- %s ---\n", title)
	fmt.Println()
}

func printRanking(sorted []kv, topN int) {
	limit := topN
	if len(sorted) < limit {
		limit = len(sorted)
	}
	for i := 0; i < limit; i++ {
		fmt.Printf("  %d. %s (%d 件)\n", i+1, sorted[i].key, sorted[i].count)
	}
}

func printHourChart(hourCounts map[int]int) {
	maxCount := 0
	for _, c := range hourCounts {
		if c > maxCount {
			maxCount = c
		}
	}
	barMaxWidth := 40
	for h := 0; h < 24; h++ {
		count := hourCounts[h]
		barLen := 0
		if maxCount > 0 {
			barLen = count * barMaxWidth / maxCount
		}
		bar := strings.Repeat("#", barLen)
		fmt.Printf("  %02d時 | %s %d\n", h, bar, count)
	}
}

func run() int {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "使い方: %s <ログファイル>\n", os.Args[0])
		return 1
	}

	filepath := os.Args[1]

	info, err := os.Stat(filepath)
	if os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "エラー: ファイルが見つかりません: %s\n", filepath)
		return 1
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		return 1
	}
	if info.Size() == 0 {
		fmt.Fprintf(os.Stderr, "エラー: ファイルが空です: %s\n", filepath)
		return 1
	}

	records, err := parseLog(filepath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "エラー: %v\n", err)
		return 1
	}
	if len(records) == 0 {
		fmt.Fprintln(os.Stderr, "パース可能なログ行が見つかりませんでした。")
		return 1
	}

	// 総リクエスト数
	printSection("総リクエスト数")
	fmt.Printf("  %d 件\n", len(records))

	// ステータスコード別件数
	statusCounts := make(map[string]int)
	for _, r := range records {
		statusCounts[r.status]++
	}
	printSection("ステータスコード別件数")
	statusKeys := make([]string, 0, len(statusCounts))
	for k := range statusCounts {
		statusKeys = append(statusKeys, k)
	}
	sort.Strings(statusKeys)
	for _, s := range statusKeys {
		fmt.Printf("  %s: %d 件\n", s, statusCounts[s])
	}

	// アクセス元 IP トップ 5
	ipCounts := make(map[string]int)
	for _, r := range records {
		ipCounts[r.ip]++
	}
	printSection("アクセス元 IP トップ 5")
	printRanking(countAndSort(ipCounts), 5)

	// リクエストパス トップ 5
	pathCounts := make(map[string]int)
	for _, r := range records {
		pathCounts[r.path]++
	}
	printSection("リクエストパス トップ 5")
	printRanking(countAndSort(pathCounts), 5)

	// 時間帯別件数
	hourCounts := make(map[int]int)
	for _, r := range records {
		hourCounts[r.hour]++
	}
	printSection("時間帯別リクエスト数")
	printHourChart(hourCounts)

	fmt.Println()
	return 0
}

func main() {
	os.Exit(run())
}
