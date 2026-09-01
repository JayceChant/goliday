// 白盒测试（package main）：经 bufconn 直接挂载未导出的 grpcServer，
// 覆盖 gRPC 语义一致性与错误场景。
package main

import (
	"context"
	"net"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"goliday"
	pb "goliday/proto/goliday/v1"
)

// newBufconnServer 从 ../../testdata 加载配置，在 bufconn 上启动 gRPC 服务
// （GolidayService + 标准健康检查，与 newGRPCServer 注册一致），
// 返回两个客户端；连接与服务端随测试结束清理。
func newBufconnServer(t *testing.T) (pb.GolidayServiceClient, grpc_health_v1.HealthClient) {
	t.Helper()
	store, err := goliday.LoadDir("../../testdata")
	if err != nil {
		t.Fatalf("加载测试配置失败: %v", err)
	}
	cal := goliday.NewCalendar(store)

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	pb.RegisterGolidayServiceServer(srv, &grpcServer{cal: cal})
	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(srv, healthSrv)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		srv.Stop()
		t.Fatalf("建立 gRPC 连接失败: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return pb.NewGolidayServiceClient(conn), grpc_health_v1.NewHealthClient(conn)
}

// wantGRPCError 断言错误码为 InvalidArgument 且 message 含指定标识
// （message 形如 "invalid_date: 非法日期 ..."）。
func wantGRPCError(t *testing.T, name string, err error, wantMarker string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s 应返回错误", name)
	}
	if got := status.Code(err); got != codes.InvalidArgument {
		t.Errorf("%s 错误码 = %v, want InvalidArgument（错误: %v）", name, got, err)
	}
	if msg := status.Convert(err).Message(); !strings.Contains(msg, wantMarker) {
		t.Errorf("%s message = %q, 应包含 %q", name, msg, wantMarker)
	}
}

// 1. GetDay 粗粒度。
func TestGRPCGetDayCoarse(t *testing.T) {
	client, _ := newBufconnServer(t)
	resp, err := client.GetDay(context.Background(), &pb.GetDayRequest{Date: "2026-02-20"})
	if err != nil {
		t.Fatalf("GetDay 失败: %v", err)
	}
	if resp.GetDate() != "2026-02-20" {
		t.Errorf("date = %q, want 2026-02-20", resp.GetDate())
	}
	if resp.GetType() != 28 {
		t.Errorf("type = %d, want 28", resp.GetType())
	}
	if resp.GetTypeLabel() != "holiday" {
		t.Errorf("type_label = %q, want holiday", resp.GetTypeLabel())
	}
	if resp.GetTotalDays() != 1 {
		t.Errorf("total_days = %d, want 1", resp.GetTotalDays())
	}
	if resp.GetStats().GetHoliday() != 1 {
		t.Errorf("stats.holiday = %d, want 1", resp.GetStats().GetHoliday())
	}
}

// 2. GetDay 细粒度。
func TestGRPCGetDayDetailed(t *testing.T) {
	client, _ := newBufconnServer(t)
	cases := []struct {
		date      string
		wantType  uint32
		wantLabel string
	}{
		{"2026-02-20", 16, "adjusted"},
		{"2026-02-17", 24, "festival|adjusted"},
		{"2026-02-28", 6, "compensate|weekend"},
	}
	for _, c := range cases {
		resp, err := client.GetDay(context.Background(), &pb.GetDayRequest{Date: c.date, Detailed: true})
		if err != nil {
			t.Fatalf("GetDay(date=%s) 失败: %v", c.date, err)
		}
		if resp.GetType() != c.wantType {
			t.Errorf("date=%s type = %d, want %d", c.date, resp.GetType(), c.wantType)
		}
		if resp.GetTypeLabel() != c.wantLabel {
			t.Errorf("date=%s type_label = %q, want %q", c.date, resp.GetTypeLabel(), c.wantLabel)
		}
		if resp.GetTotalDays() != 1 {
			t.Errorf("date=%s total_days = %d, want 1", c.date, resp.GetTotalDays())
		}
	}
}

// 3. QueryDays 区间查询与粗粒度统计。
func TestGRPCQueryDaysRange(t *testing.T) {
	client, _ := newBufconnServer(t)
	resp, err := client.QueryDays(context.Background(),
		&pb.QueryDaysRequest{Start: "2026-02-14", End: "2026-02-17"})
	if err != nil {
		t.Fatalf("QueryDays 失败: %v", err)
	}
	if resp.GetMode() != "range" {
		t.Errorf("mode = %q, want range", resp.GetMode())
	}
	if resp.GetStart() != "2026-02-14" || resp.GetEnd() != "2026-02-17" {
		t.Errorf("start/end = %q/%q, want 2026-02-14/2026-02-17", resp.GetStart(), resp.GetEnd())
	}
	if resp.GetTotalDays() != 3 {
		t.Errorf("total_days = %d, want 3", resp.GetTotalDays())
	}
	if d := resp.GetDays()[0]; d.GetDate() != "2026-02-14" || d.GetType() != 6 {
		t.Errorf("days[0] = %s/%d, want 2026-02-14/6（02-14 补班）", d.GetDate(), d.GetType())
	}
	if d := resp.GetDays()[2]; d.GetDate() != "2026-02-16" || d.GetType() != 16 {
		t.Errorf("days[2] = %s/%d, want 2026-02-16/16（02-16 调休）", d.GetDate(), d.GetType())
	}
	st := resp.GetStats()
	if st.GetWorkday() != 1 {
		t.Errorf("stats.workday = %d, want 1", st.GetWorkday())
	}
	if st.GetHoliday() != 2 {
		t.Errorf("stats.holiday = %d, want 2", st.GetHoliday())
	}
}

// 4. QueryDays 离散列表：去重升序。
func TestGRPCQueryDaysListDedupSorted(t *testing.T) {
	client, _ := newBufconnServer(t)
	resp, err := client.QueryDays(context.Background(), &pb.QueryDaysRequest{
		Dates: []string{"2026-02-28", "2026-02-16", "2026-02-17", "2026-02-16"}})
	if err != nil {
		t.Fatalf("QueryDays 失败: %v", err)
	}
	if resp.GetMode() != "list" {
		t.Errorf("mode = %q, want list", resp.GetMode())
	}
	if resp.GetTotalDays() != 3 {
		t.Errorf("total_days = %d, want 3", resp.GetTotalDays())
	}
	want := []struct {
		date string
		typ  uint32
	}{
		{"2026-02-16", 16},
		{"2026-02-17", 24},
		{"2026-02-28", 6},
	}
	days := resp.GetDays()
	if len(days) != len(want) {
		t.Fatalf("days 长度 = %d, want %d", len(days), len(want))
	}
	for i, w := range want {
		if days[i].GetDate() != w.date || days[i].GetType() != w.typ {
			t.Errorf("days[%d] = %s/%d, want %s/%d", i, days[i].GetDate(), days[i].GetType(), w.date, w.typ)
		}
	}
}

// 5. QueryDays 区间与列表混合：并集。
func TestGRPCQueryDaysMixed(t *testing.T) {
	client, _ := newBufconnServer(t)
	resp, err := client.QueryDays(context.Background(), &pb.QueryDaysRequest{
		Start: "2026-02-01", End: "2026-02-03", Dates: []string{"2026-03-08"}})
	if err != nil {
		t.Fatalf("QueryDays 失败: %v", err)
	}
	if resp.GetMode() != "list" {
		t.Errorf("mode = %q, want list", resp.GetMode())
	}
	if resp.GetTotalDays() != 3 {
		t.Errorf("total_days = %d, want 3", resp.GetTotalDays())
	}
	want := []string{"2026-02-01", "2026-02-02", "2026-03-08"}
	days := resp.GetDays()
	if len(days) != len(want) {
		t.Fatalf("days 长度 = %d, want %d", len(days), len(want))
	}
	for i, w := range want {
		if days[i].GetDate() != w {
			t.Errorf("days[%d].date = %q, want %q", i, days[i].GetDate(), w)
		}
	}
}

// 6. QueryStats 与 QueryDays 口径一致，且响应无 days 明细。
func TestGRPCQueryStatsMatchesDays(t *testing.T) {
	client, _ := newBufconnServer(t)
	req := &pb.QueryDaysRequest{Start: "2026-02-14", End: "2026-02-17"}
	daysResp, err := client.QueryDays(context.Background(), req)
	if err != nil {
		t.Fatalf("QueryDays 失败: %v", err)
	}
	statsResp, err := client.QueryStats(context.Background(), req)
	if err != nil {
		t.Fatalf("QueryStats 失败: %v", err)
	}
	if statsResp.GetMode() != daysResp.GetMode() {
		t.Errorf("mode 不一致: %q vs %q", statsResp.GetMode(), daysResp.GetMode())
	}
	if statsResp.GetTotalDays() != daysResp.GetTotalDays() {
		t.Errorf("total_days 不一致: %d vs %d", statsResp.GetTotalDays(), daysResp.GetTotalDays())
	}
	if statsResp.GetStart() != daysResp.GetStart() || statsResp.GetEnd() != daysResp.GetEnd() {
		t.Errorf("start/end 不一致: %q/%q vs %q/%q",
			statsResp.GetStart(), statsResp.GetEnd(), daysResp.GetStart(), daysResp.GetEnd())
	}
	want, got := daysResp.GetStats(), statsResp.GetStats()
	if want.GetWorkday() != got.GetWorkday() || want.GetHoliday() != got.GetHoliday() {
		t.Errorf("stats 不一致: %v vs %v", got, want)
	}
	// StatsResponse 消息本身不含 days 字段（proto 契约静态保证，无法携带明细）。
}

// 7. detailed 统计交叉计数：组合日对每个标志位各计 1。
func TestGRPCFineStatsCrossCount(t *testing.T) {
	client, _ := newBufconnServer(t)
	resp, err := client.QueryDays(context.Background(), &pb.QueryDaysRequest{
		Dates: []string{"2026-02-17", "2026-02-28"}, Detailed: true})
	if err != nil {
		t.Fatalf("QueryDays 失败: %v", err)
	}
	st := resp.GetStats()
	if st.GetFestival() != 1 || st.GetAdjusted() != 1 || st.GetCompensate() != 1 || st.GetWeekend() != 1 {
		t.Errorf("细粒度计数 = festival:%d adjusted:%d compensate:%d weekend:%d, want 各 1",
			st.GetFestival(), st.GetAdjusted(), st.GetCompensate(), st.GetWeekend())
	}
	if st.GetOrdinary() != 0 {
		t.Errorf("stats.ordinary = %d, want 0", st.GetOrdinary())
	}
	if st.GetWorkday() != 1 {
		t.Errorf("stats.workday = %d, want 1（02-28 补班归上班日）", st.GetWorkday())
	}
	if st.GetHoliday() != 1 {
		t.Errorf("stats.holiday = %d, want 1（02-17 调休归休息日）", st.GetHoliday())
	}
}

// 8. 参数错误统一 InvalidArgument，message 含对应标识。
func TestGRPCInvalidArgument(t *testing.T) {
	client, _ := newBufconnServer(t)
	ctx := context.Background()

	_, err := client.GetDay(ctx, &pb.GetDayRequest{Date: "2026-02-30"})
	wantGRPCError(t, "GetDay(2026-02-30)", err, "invalid_date")

	_, err = client.QueryDays(ctx, &pb.QueryDaysRequest{Start: "2026-03-05", End: "2026-03-01"})
	wantGRPCError(t, "QueryDays(end<start)", err, "invalid_range")

	_, err = client.QueryDays(ctx, &pb.QueryDaysRequest{Start: "2026-01-01", End: "2027-06-01"})
	wantGRPCError(t, "QueryDays(跨度>366)", err, "invalid_range")

	_, err = client.QueryDays(ctx, &pb.QueryDaysRequest{})
	wantGRPCError(t, "QueryDays(全空)", err, "missing_query")
}

// 9. gRPC 标准健康检查：整体状态 SERVING。
func TestGRPCHealthCheck(t *testing.T) {
	_, healthClient := newBufconnServer(t)
	resp, err := healthClient.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("健康检查失败: %v", err)
	}
	if got := resp.GetStatus(); got != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Errorf("健康状态 = %v, want SERVING", got)
	}
}

// 10. 未加载年份：GetDay/QueryDays/QueryStats 均返回 InvalidArgument
// year_not_loaded，message 含年份（与 HTTP 同源）。
func TestGRPCYearNotLoaded(t *testing.T) {
	client, _ := newBufconnServer(t)
	ctx := context.Background()

	_, err := client.GetDay(ctx, &pb.GetDayRequest{Date: "2027-05-01"})
	wantGRPCError(t, "GetDay(2027-05-01)", err, "year_not_loaded")
	if msg := status.Convert(err).Message(); !strings.Contains(msg, "2027") {
		t.Errorf("GetDay message = %q, 应包含 2027", msg)
	}

	// QueryStats 跨年区间含未加载中间年（2025 已加载、2026 已加载，
	// 用 2025→2028 验证多年份列举）。
	_, err = client.QueryStats(ctx, &pb.QueryDaysRequest{Start: "2025-06-01", End: "2028-12-31"})
	wantGRPCError(t, "QueryStats(2025→2028)", err, "year_not_loaded")
	msg := status.Convert(err).Message()
	for _, want := range []string{"2027", "2028"} {
		if !strings.Contains(msg, want) {
			t.Errorf("QueryStats message = %q, 应包含 %q", msg, want)
		}
	}

	// 离散列表含未加载年。
	_, err = client.QueryDays(ctx, &pb.QueryDaysRequest{Dates: []string{"2026-02-17", "2029-01-01"}})
	wantGRPCError(t, "QueryDays(列表含 2029)", err, "year_not_loaded")
}

// 11. QueryStats 不限跨度（前缀和）：跨 2025→2026 大区间成功且与分段一致；
// QueryDays 同区间仍被 366 上限拒绝；空区间 200 全零。
func TestGRPCQueryStatsLargeRangeAndEmptyRange(t *testing.T) {
	client, _ := newBufconnServer(t)
	ctx := context.Background()

	resp, err := client.QueryStats(ctx, &pb.QueryDaysRequest{Start: "2025-01-01", End: "2027-01-01"})
	if err != nil {
		t.Fatalf("QueryStats 大跨度失败: %v", err)
	}
	if resp.GetTotalDays() != 730 {
		t.Errorf("total_days = %d, want 730", resp.GetTotalDays())
	}
	st := resp.GetStats()
	if st.GetWorkday()+st.GetHoliday() != 730 {
		t.Errorf("workday+holiday = %d, want 730", st.GetWorkday()+st.GetHoliday())
	}

	// 分段对照。
	y25, err := client.QueryStats(ctx, &pb.QueryDaysRequest{Start: "2025-01-01", End: "2026-01-01"})
	if err != nil {
		t.Fatalf("QueryStats(2025) 失败: %v", err)
	}
	y26, err := client.QueryStats(ctx, &pb.QueryDaysRequest{Start: "2026-01-01", End: "2027-01-01"})
	if err != nil {
		t.Fatalf("QueryStats(2026) 失败: %v", err)
	}
	if st.GetWorkday() != y25.GetStats().GetWorkday()+y26.GetStats().GetWorkday() {
		t.Errorf("workday 分段不一致: %d vs %d+%d",
			st.GetWorkday(), y25.GetStats().GetWorkday(), y26.GetStats().GetWorkday())
	}

	// QueryDays 同区间：超 366 天被拒。
	_, err = client.QueryDays(ctx, &pb.QueryDaysRequest{Start: "2025-01-01", End: "2027-01-01"})
	wantGRPCError(t, "QueryDays(跨度>366)", err, "invalid_range")

	// 空区间（2027 未加载）：无覆盖年份，全零成功。
	empty, err := client.QueryStats(ctx, &pb.QueryDaysRequest{Start: "2027-01-01", End: "2027-01-01"})
	if err != nil {
		t.Fatalf("QueryStats 空区间失败: %v", err)
	}
	if empty.GetTotalDays() != 0 || empty.GetStats().GetWorkday() != 0 || empty.GetStats().GetHoliday() != 0 {
		t.Errorf("空区间结果 = %v, want 全零", empty)
	}
}
