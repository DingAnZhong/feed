package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const baseURL = "http://127.0.0.1:8080"

type PublishReq struct {
	Content   string   `json:"content"`
	MediaUrls []string `json:"media_urls"`
}

type FollowReq struct {
	FolloweeID int64 `json:"followee_id"`
	ActionType int   `json:"action_type"`
}

type ApiResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

type FeedResponseData struct {
	NextTime int64  `json:"next_time"`
	IsEnd    bool   `json:"is_end"`
	Posts    []Post `json:"post_list"`
}

type Post struct {
	PostID    int64    `json:"post_id"`
	Author    UserInfo `json:"author"`
	Content   string   `json:"content"`
	MediaUrls []string `json:"media_urls"`
	CreatedAt int64    `json:"create_time"`
}

type UserInfo struct {
	UserID   int64  `json:"user_id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

type BenchmarkResult struct {
	TotalRequests  int64
	SuccessCount   int64
	FailCount      int64
	TotalDuration  time.Duration
	AvgLatency     time.Duration
	MinLatency     time.Duration
	MaxLatency     time.Duration
	P50Latency     time.Duration
	P90Latency     time.Duration
	P95Latency     time.Duration
	P99Latency     time.Duration
	RequestsPerSec float64
	Errors         map[string]int
}

func (r *BenchmarkResult) Print() {
	fmt.Println("\n============================================================")
	fmt.Println("                         压测报告")
	fmt.Println("============================================================")
	fmt.Printf("  总请求数:      %d\n", r.TotalRequests)
	fmt.Printf("  成功数:        %d (%.2f%%)\n", r.SuccessCount, float64(r.SuccessCount)/float64(r.TotalRequests)*100)
	fmt.Printf("  失败数:        %d (%.2f%%)\n", r.FailCount, float64(r.FailCount)/float64(r.TotalRequests)*100)
	fmt.Printf("  总耗时:        %s\n", r.TotalDuration)
	fmt.Printf("  吞吐量:        %.2f req/s\n", r.RequestsPerSec)
	fmt.Printf("\n--- 延迟统计 ---\n")
	fmt.Printf("  平均延迟:      %s\n", r.AvgLatency)
	fmt.Printf("  最小延迟:      %s\n", r.MinLatency)
	fmt.Printf("  最大延迟:      %s\n", r.MaxLatency)
	fmt.Printf("  P50 延迟:      %s\n", r.P50Latency)
	fmt.Printf("  P90 延迟:      %s\n", r.P90Latency)
	fmt.Printf("  P95 延迟:      %s\n", r.P95Latency)
	fmt.Printf("  P99 延迟:      %s\n", r.P99Latency)
	if len(r.Errors) > 0 {
		fmt.Printf("\n--- 错误分布 ---\n")
		for err, count := range r.Errors {
			fmt.Printf("  %-40s: %d\n", err, count)
		}
	}
	fmt.Println("============================================================")
}

type HTTPClient struct {
	client *http.Client
}

func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (hc *HTTPClient) PostPublish(userID int64, req PublishReq) (*ApiResponse, time.Duration, error) {
	body, _ := json.Marshal(req)
	start := time.Now()
	resp, err := hc.client.Post(baseURL+"/api/v1/post/publish", "application/json", bytes.NewBuffer(body))
	latency := time.Since(start)
	if err != nil {
		return nil, latency, err
	}
	defer resp.Body.Close()
	var apiResp ApiResponse
	respBytes, _ := io.ReadAll(resp.Body)
	json.Unmarshal(respBytes, &apiResp)
	return &apiResp, latency, nil
}

func (hc *HTTPClient) Follow(userID int64, req FollowReq) (*ApiResponse, time.Duration, error) {
	start := time.Now()
	reqStr := fmt.Sprintf(`{"followee_id":%d,"action_type":%d}`, req.FolloweeID, req.ActionType)
	resp, err := hc.client.Post(baseURL+"/api/v1/user/follow", "application/json", bytes.NewBuffer([]byte(reqStr)))
	latency := time.Since(start)
	if err != nil {
		return nil, latency, err
	}
	defer resp.Body.Close()
	var apiResp ApiResponse
	respBytes, _ := io.ReadAll(resp.Body)
	json.Unmarshal(respBytes, &apiResp)
	return &apiResp, latency, nil
}

func (hc *HTTPClient) FetchFeed(userID int64, latestTime int64, limit int) (*ApiResponse, time.Duration, error) {
	start := time.Now()
	url := fmt.Sprintf("%s/api/v1/feed/timeline?latest_time=%d&limit=%d", baseURL, latestTime, limit)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-User-ID", strconv.FormatInt(userID, 10))
	resp, err := hc.client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return nil, latency, err
	}
	defer resp.Body.Close()
	var apiResp ApiResponse
	respBytes, _ := io.ReadAll(resp.Body)
	json.Unmarshal(respBytes, &apiResp)
	return &apiResp, latency, nil
}

type Scenario interface {
	Name() string
	Run(result *BenchmarkResult)
}

type PublishScenario struct{ UserIDs []int64 }

func (s *PublishScenario) Name() string { return "发帖压测" }

func (s *PublishScenario) Run(result *BenchmarkResult) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var latencies []time.Duration
	var minLatency, maxLatency time.Duration
	client := NewHTTPClient()
	for _, userID := range s.UserIDs {
		wg.Add(1)
		go func(uid int64) {
			defer wg.Done()
			content := fmt.Sprintf("压测帖子内容 - 用户%d", uid)
			resp, latency, err := client.PostPublish(uid, PublishReq{Content: content, MediaUrls: []string{"https://cdn.example.com/img.jpg"}})
			mu.Lock()
			latencies = append(latencies, latency)
			if latency < minLatency || minLatency == 0 {
				minLatency = latency
			}
			if latency > maxLatency {
				maxLatency = latency
			}
			if err != nil {
				result.FailCount++
				result.Errors[err.Error()]++
			} else if resp.Code == 0 {
				result.SuccessCount++
			} else {
				result.FailCount++
				result.Errors[resp.Msg]++
			}
			mu.Unlock()
		}(userID)
	}
	wg.Wait()
	sortTimes(latencies)
	avgLatency := calcAvg(latencies)
	result.MinLatency, result.MaxLatency = minLatency, maxLatency
	result.AvgLatency = avgLatency
	result.P50Latency = getPercentile(latencies, 50)
	result.P90Latency = getPercentile(latencies, 90)
	result.P95Latency = getPercentile(latencies, 95)
	result.P99Latency = getPercentile(latencies, 99)
}

type FeedFetchScenario struct{ UserIDs []int64 }

func (s *FeedFetchScenario) Name() string { return "Feed流拉取压测" }

func (s *FeedFetchScenario) Run(result *BenchmarkResult) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var latencies []time.Duration
	var minLatency, maxLatency time.Duration
	client := NewHTTPClient()
	for _, userID := range s.UserIDs {
		wg.Add(1)
		go func(uid int64) {
			defer wg.Done()
			resp, latency, err := client.FetchFeed(uid, 0, 10)
			mu.Lock()
			latencies = append(latencies, latency)
			if latency < minLatency || minLatency == 0 {
				minLatency = latency
			}
			if latency > maxLatency {
				maxLatency = latency
			}
			if err != nil {
				result.FailCount++
				result.Errors[err.Error()]++
			} else if resp.Code == 0 {
				result.SuccessCount++
			} else {
				result.FailCount++
				result.Errors[resp.Msg]++
			}
			mu.Unlock()
		}(userID)
	}
	wg.Wait()
	sortTimes(latencies)
	avgLatency := calcAvg(latencies)
	result.MinLatency, result.MaxLatency = minLatency, maxLatency
	result.AvgLatency = avgLatency
	result.P50Latency = getPercentile(latencies, 50)
	result.P90Latency = getPercentile(latencies, 90)
	result.P95Latency = getPercentile(latencies, 95)
	result.P99Latency = getPercentile(latencies, 99)
}

type FollowScenario struct {
	UserIDs  []int64
	TargetID int64
}

func (s *FollowScenario) Name() string { return "关注操作压测" }

func (s *FollowScenario) Run(result *BenchmarkResult) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var latencies []time.Duration
	var minLatency, maxLatency time.Duration
	client := NewHTTPClient()
	for _, userID := range s.UserIDs {
		wg.Add(1)
		go func(uid int64) {
			defer wg.Done()
			resp, latency, err := client.Follow(uid, FollowReq{FolloweeID: s.TargetID, ActionType: 1})
			mu.Lock()
			latencies = append(latencies, latency)
			if latency < minLatency || minLatency == 0 {
				minLatency = latency
			}
			if latency > maxLatency {
				maxLatency = latency
			}
			if err != nil {
				result.FailCount++
				result.Errors[err.Error()]++
			} else if resp.Code == 0 {
				result.SuccessCount++
			} else {
				result.FailCount++
				result.Errors[resp.Msg]++
			}
			mu.Unlock()
		}(userID)
	}
	wg.Wait()
	sortTimes(latencies)
	avgLatency := calcAvg(latencies)
	result.MinLatency, result.MaxLatency = minLatency, maxLatency
	result.AvgLatency = avgLatency
	result.P50Latency = getPercentile(latencies, 50)
	result.P90Latency = getPercentile(latencies, 90)
	result.P95Latency = getPercentile(latencies, 95)
	result.P99Latency = getPercentile(latencies, 99)
}

type MixedScenario struct {
	PublishRatio int
	FeedRatio    int
	FollowRatio  int
	UserIDs      []int64
	TargetID     int64
}

func (s *MixedScenario) Name() string { return "混合并发压测" }

func (s *MixedScenario) Run(result *BenchmarkResult) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var latencies []time.Duration
	var minLatency, maxLatency time.Duration
	client := NewHTTPClient()
	totalOps := s.PublishRatio + s.FeedRatio + s.FollowRatio
	for i := 0; i < totalOps; i++ {
		op := i % totalOps
		userIdx := i % len(s.UserIDs)
		wg.Add(1)
		go func() {
			defer wg.Done()
			var resp *ApiResponse
			var latency time.Duration
			var err error
			if op == 0 {
				resp, latency, err = client.PostPublish(s.UserIDs[userIdx], PublishReq{Content: fmt.Sprintf("混合压测帖子 %d", userIdx), MediaUrls: []string{}})
			} else if op == 1 {
				resp, latency, err = client.FetchFeed(s.UserIDs[userIdx], 0, 10)
			} else {
				resp, latency, err = client.Follow(s.UserIDs[userIdx], FollowReq{FolloweeID: s.TargetID, ActionType: 1})
			}
			mu.Lock()
			latencies = append(latencies, latency)
			if latency < minLatency || minLatency == 0 {
				minLatency = latency
			}
			if latency > maxLatency {
				maxLatency = latency
			}
			if err != nil {
				result.FailCount++
				result.Errors[err.Error()]++
			} else if resp.Code == 0 {
				result.SuccessCount++
			} else {
				result.FailCount++
				result.Errors[resp.Msg]++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	sortTimes(latencies)
	avgLatency := calcAvg(latencies)
	result.MinLatency, result.MaxLatency = minLatency, maxLatency
	result.AvgLatency = avgLatency
	result.P50Latency = getPercentile(latencies, 50)
	result.P90Latency = getPercentile(latencies, 90)
	result.P95Latency = getPercentile(latencies, 95)
	result.P99Latency = getPercentile(latencies, 99)
}

func sortTimes(times []time.Duration) {
	for i := 1; i < len(times); i++ {
		for j := i; j > 0 && times[j] < times[j-1]; j-- {
			times[j], times[j-1] = times[j-1], times[j]
		}
	}
}

func calcAvg(times []time.Duration) time.Duration {
	if len(times) == 0 {
		return 0
	}
	total := time.Duration(0)
	for _, t := range times {
		total += t
	}
	return time.Duration(int64(total) / int64(len(times)))
}

func getPercentile(times []time.Duration, percentile float64) time.Duration {
	if len(times) == 0 {
		return 0
	}
	index := int(math.Floor(float64(len(times)) * percentile / 100))
	if index >= len(times) {
		index = len(times) - 1
	}
	return times[index]
}

func main() {
	fmt.Println("============================================================")
	fmt.Println("              Feed 流系统 - 压测工具")
	fmt.Println("============================================================")
	fmt.Printf("  目标地址: %s\n", baseURL)
	fmt.Println("============================================================")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/ping")
	if err != nil {
		fmt.Printf("无法连接到 %s，请确保服务已启动\n", baseURL)
		fmt.Printf("  错误: %v\n", err)
		fmt.Println("\n请先运行: go run cmd/api/main.go")
		return
	}
	resp.Body.Close()
	fmt.Println("服务连接成功")

	userIDs := make([]int64, 0, 100)
	for i := 0; i < 100; i++ {
		userIDs = append(userIDs, int64(i+1))
	}

	scenarios := []Scenario{
		&PublishScenario{UserIDs: userIDs},
		&FeedFetchScenario{UserIDs: userIDs},
		&FollowScenario{UserIDs: userIDs, TargetID: 1},
		&MixedScenario{PublishRatio: 2, FeedRatio: 5, FollowRatio: 1, UserIDs: userIDs, TargetID: 1},
	}

	for idx, scenario := range scenarios {
		fmt.Printf("\n场景 %d: %s\n", idx+1, scenario.Name())
		fmt.Println("------------------------------------------------------------")
		result := &BenchmarkResult{Errors: make(map[string]int)}
		startTime := time.Now()
		scenario.Run(result)
		result.TotalDuration = time.Since(startTime)
		result.TotalRequests = result.SuccessCount + result.FailCount
		if result.TotalDuration > 0 {
			result.RequestsPerSec = float64(result.TotalRequests) / result.TotalDuration.Seconds()
		}
		result.Print()
	}
	fmt.Println("\n全部压测完成!")
}

// README: 压测步骤
// 1. 确保 MySQL / Redis / Kafka 服务已启动
// 2. 初始化测试数据: go run test/init_data.go (如果文件存在)
// 3. 启动 API 服务: go run cmd/api/main.go
// 4. 启动 Worker 服务: go run cmd/worker/main.go
// 5. 运行压测: go run test/benchmark.go
// 6. 修改 baseURL 常量可指向不同环境
