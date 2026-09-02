// gRPC 服务实现：GolidayService 三方法与 grpc 标准健康检查，与 HTTP 同进程。
//
// 查询语义与 HTTP 接口完全一致：参数校验与归一化复用 handlers.go 的
// parseDate/resolveMultiQuery/checkYears（含 errMissingQuery、
// year_not_loaded 等共用错误文案），统计统一走前缀和路径
// statsFor（区间 StatsRange 差分 + 列表剔除区间内日期后分段统计）；
// 参数错误统一映射为 codes.InvalidArgument，message 为
// "错误码标识: HTTP 同源文案"。
package main

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	"github.com/JayceChant/goliday"
	pb "github.com/JayceChant/goliday/proto/goliday/v1"
)

// grpcServer GolidayService 服务端实现，持有只读日历索引，
// 构造完成后可被多个 goroutine 并发调用。
type grpcServer struct {
	pb.UnimplementedGolidayServiceServer
	cal *goliday.Calendar
}

// newGRPCServer 创建 gRPC 服务端：注册 GolidayService 与 grpc 标准健康检查
// 服务（整体状态置为 SERVING）。返回值由调用方负责 Serve 与 GracefulStop。
func newGRPCServer(cal *goliday.Calendar) *grpc.Server {
	srv := grpc.NewServer()
	pb.RegisterGolidayServiceServer(srv, &grpcServer{cal: cal})
	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(srv, healthSrv)
	return srv
}

// grpcErr 将统一 API 错误映射为 gRPC 错误：HTTP 400 类参数错误统一对应
// InvalidArgument，message 以错误码标识前缀 HTTP 同源文案，保证两个协议
// 提示一致且可按标识检索。
func grpcErr(aerr *apiError) error {
	return status.Error(codes.InvalidArgument, aerr.code+": "+aerr.message)
}

// GetDay 单日查询，语义同 HTTP GET /api/v1/days?date=...：
// detailed=false（默认）返回粗粒度类型（workday/holiday），
// true 返回细粒度掩码与 DayType.String()；total_days 恒为 1。
// date 所在年份未加载时返回 year_not_loaded（InvalidArgument）。
func (s *grpcServer) GetDay(_ context.Context, req *pb.GetDayRequest) (*pb.GetDayResponse, error) {
	if req.GetDate() == "" {
		return nil, grpcErr(errMissingQuery)
	}
	d, aerr := parseDate(req.GetDate())
	if aerr != nil {
		return nil, grpcErr(aerr)
	}
	if aerr := checkYears(s.cal, []int{d.Year()}); aerr != nil {
		return nil, grpcErr(aerr)
	}
	detailed := req.GetDetailed()
	shown, err := s.cal.Query(d)
	if err != nil {
		return nil, grpcErr(errInternalQuery)
	}
	if !detailed {
		shown = shown.Coarse()
	}
	st, err := s.cal.Stats([]time.Time{d}, detailed)
	if err != nil {
		return nil, grpcErr(errInternalQuery)
	}
	return &pb.GetDayResponse{
		Date:      d.Format(dateLayout),
		Type:      uint32(shown),
		TypeLabel: shown.String(),
		TotalDays: int32(st.Total),
		Stats:     statsProto(st, detailed),
	}, nil
}

// queryMulti QueryDays 与 QueryStats 共用的查询实现：归一化入参并校验
// 覆盖年份已加载（year_not_loaded），随后统一走前缀和统计。
// days 明细恒为细粒度数值，由 QueryDays 负责填充。
func (s *grpcServer) queryMulti(start, end string, dates []string, detailed, wantDays bool) (*multiQuery, goliday.StatsResult, error) {
	// 仅 QueryDays（含明细）限制区间跨度；QueryStats 基于前缀和不限跨度。
	maxSpan := 0
	if wantDays {
		maxSpan = maxRangeDays
	}
	mq, aerr := resolveMultiQuery(start, end, dates, maxSpan)
	if aerr != nil {
		return nil, goliday.StatsResult{}, grpcErr(aerr)
	}
	if aerr := checkYears(s.cal, mq.coveredYears()); aerr != nil {
		return nil, goliday.StatsResult{}, grpcErr(aerr)
	}
	st, aerr := statsFor(s.cal, mq, detailed)
	if aerr != nil {
		return nil, goliday.StatsResult{}, grpcErr(aerr)
	}
	return mq, st, nil
}

// QueryDays 区间/离散/混合查询，语义同 HTTP GET /api/v1/days 的多日模式：
// 区间左闭右开且跨度 ≤366 天、离散去重升序、并存取并集（mode=list）；
// 覆盖年份未加载时返回 year_not_loaded（InvalidArgument）。
func (s *grpcServer) QueryDays(_ context.Context, req *pb.QueryDaysRequest) (*pb.QueryDaysResponse, error) {
	mq, st, err := s.queryMulti(req.GetStart(), req.GetEnd(), req.GetDates(), req.GetDetailed(), true)
	if err != nil {
		return nil, err
	}
	dates := mq.datesForDays()
	days := make([]*pb.Day, len(dates))
	for i, d := range dates {
		t, qerr := s.cal.Query(d)
		if qerr != nil {
			return nil, grpcErr(errInternalQuery)
		}
		days[i] = &pb.Day{Date: d.Format(dateLayout), Type: uint32(t), TypeLabel: t.String()}
	}
	return &pb.QueryDaysResponse{
		Mode:      mq.mode,
		Start:     mq.start,
		End:       mq.end,
		TotalDays: int32(st.Total),
		Days:      days,
		Stats:     statsProto(st, req.GetDetailed()),
	}, nil
}

// QueryStats 统计查询，入参结构与统计口径同 QueryDays（不限跨度），但不
// 返回 days 明细；按 buf lint 要求使用独立消息，字段与 QueryDaysRequest 同构。
func (s *grpcServer) QueryStats(_ context.Context, req *pb.QueryStatsRequest) (*pb.QueryStatsResponse, error) {
	mq, st, err := s.queryMulti(req.GetStart(), req.GetEnd(), req.GetDates(), req.GetDetailed(), false)
	if err != nil {
		return nil, err
	}
	return &pb.QueryStatsResponse{
		Mode:      mq.mode,
		Start:     mq.start,
		End:       mq.end,
		TotalDays: int32(st.Total),
		Stats:     statsProto(st, req.GetDetailed()),
	}, nil
}

// statsProto 将统计结果转为 Stats 消息：workday/holiday 恒填充（粗粒度），
// 五个细粒度键仅 detailed=true 时填充（五键 MECE，之和恒等于总天数）。
func statsProto(st goliday.StatsResult, detailed bool) *pb.Stats {
	p := &pb.Stats{
		Workday: int32(st.Coarse[goliday.DayTypeWork]),
		Holiday: int32(st.Coarse[goliday.DayTypeRest]),
	}
	if detailed {
		p.Ordinary = int32(st.Fine[goliday.DayTypeWork])
		p.Compensate = int32(st.Fine[goliday.DayTypeCompensate])
		p.Weekend = int32(st.Fine[goliday.DayTypeRest])
		p.Festival = int32(st.Fine[goliday.DayTypeFestival])
		p.Adjusted = int32(st.Fine[goliday.DayTypeAdjusted])
	}
	return p
}
