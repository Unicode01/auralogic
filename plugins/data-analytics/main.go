package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"strings"
	"time"

	"auralogic-plugin-data-analytics/pb"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedPluginServiceServer
}

func (s *server) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	return &pb.HealthCheckResponse{
		Healthy: true,
		Version: "1.0.0",
		Metadata: map[string]string{
			"plugin": "data-analytics",
		},
	}, nil
}

func (s *server) Execute(ctx context.Context, req *pb.ExecuteRequest) (*pb.ExecuteResponse, error) {
	switch req.Action {
	case "hook.execute":
		return s.executeHook(req)
	case "analyze_orders":
		period := req.GetParams()["period"]
		if period == "" {
			period = "7d"
		}
		data, _ := json.Marshal(map[string]interface{}{
			"total_orders": 100,
			"paid_orders":  98,
			"trend":        "up",
			"period":       period,
			"generated_at": time.Now().Format(time.RFC3339),
			"source":       "data-analytics-example",
		})
		return &pb.ExecuteResponse{
			Success: true,
			Data:    string(data),
			Metadata: map[string]string{
				"plugin": "data-analytics",
				"action": "analyze_orders",
			},
		}, nil
	case "health_check":
		return &pb.ExecuteResponse{
			Success: true,
			Data:    `{"status":"ok"}`,
		}, nil
	default:
		return &pb.ExecuteResponse{
			Success: false,
			Error:   "unknown action",
		}, nil
	}
}

func (s *server) executeHook(req *pb.ExecuteRequest) (*pb.ExecuteResponse, error) {
	hook := strings.TrimSpace(req.GetParams()["hook"])
	payload := map[string]interface{}{}
	if payloadRaw := strings.TrimSpace(req.GetParams()["payload"]); payloadRaw != "" {
		if err := json.Unmarshal([]byte(payloadRaw), &payload); err != nil {
			return &pb.ExecuteResponse{
				Success: false,
				Error:   "invalid hook payload",
			}, nil
		}
	}

	var result map[string]interface{}
	switch hook {
	case "order.create.before":
		promoCode, _ := payload["promo_code"].(string)
		remark, _ := payload["remark"].(string)
		if strings.EqualFold(strings.TrimSpace(promoCode), "BLOCKME") {
			result = map[string]interface{}{
				"blocked":      true,
				"block_reason": "Promo code BLOCKME is blocked by plugin policy",
			}
		} else if strings.TrimSpace(remark) == "" {
			result = map[string]interface{}{
				"payload": map[string]interface{}{
					"remark": "[Plugin] auto-tagged order",
				},
			}
		} else {
			result = map[string]interface{}{}
		}
	case "order.create.after":
		result = map[string]interface{}{
			"payload": map[string]interface{}{
				"processed_by_plugin": "data-analytics-example",
				"processed_at":        time.Now().Format(time.RFC3339),
			},
		}
	case "frontend.slot.render":
		slot, _ := payload["slot"].(string)
		pagePath, _ := payload["path"].(string)
		result = map[string]interface{}{
			"frontend_extensions": buildFrontendExtensions(slot, pagePath),
		}
	default:
		return &pb.ExecuteResponse{
			Success: false,
			Error:   "unknown hook",
		}, nil
	}

	data, _ := json.Marshal(result)
	return &pb.ExecuteResponse{
		Success: true,
		Data:    string(data),
		Metadata: map[string]string{
			"plugin": "data-analytics",
			"action": "hook.execute",
			"hook":   hook,
		},
	}, nil
}

func buildFrontendExtensions(slot, pagePath string) []map[string]interface{} {
	switch slot {
	case "user.cart.top":
		return []map[string]interface{}{
			{
				"id":       "cart-top-banner",
				"slot":     slot,
				"type":     "banner",
				"title":    "Plugin Campaign",
				"content":  "Buy 2 or more items to unlock a surprise gift.",
				"priority": 10,
				"data": map[string]interface{}{
					"path": pagePath,
				},
			},
		}
	case "user.cart.before_checkout":
		return []map[string]interface{}{
			{
				"id":       "cart-help-links",
				"slot":     slot,
				"type":     "link_list",
				"title":    "Need Help Before Checkout?",
				"priority": 20,
				"data": map[string]interface{}{
					"links": []map[string]interface{}{
						{"label": "Shipping Policy", "url": "/help/shipping"},
						{"label": "Refund Policy", "url": "/help/refund"},
					},
				},
			},
		}
	case "admin.dashboard.top":
		return []map[string]interface{}{
			{
				"id":       "admin-dashboard-top",
				"slot":     slot,
				"type":     "banner",
				"title":    "Plugin Insight",
				"content":  "Traffic is up 8.6% this week (sample data from plugin).",
				"priority": 5,
			},
		}
	case "admin.dashboard.bottom":
		return []map[string]interface{}{
			{
				"id":       "admin-dashboard-bottom",
				"slot":     slot,
				"type":     "html",
				"priority": 50,
				"content":  `<div style="padding:12px;border:1px dashed #94a3b8;border-radius:8px;font-size:13px;">Rendered by gRPC plugin on path: <strong>` + pagePath + `</strong></div>`,
			},
		}
	default:
		return nil
	}
}

func (s *server) ExecuteStream(req *pb.ExecuteRequest, stream pb.PluginService_ExecuteStreamServer) error {
	if err := stream.Send(&pb.ExecuteResponse{
		Success: false,
		Data:    `{"status":"running","progress":50}`,
		IsFinal: false,
	}); err != nil {
		return err
	}
	return stream.Send(&pb.ExecuteResponse{
		Success: true,
		Data:    `{"status":"completed","progress":100}`,
		IsFinal: true,
	})
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPluginServiceServer(grpcServer, &server{})

	log.Println("Data Analytics Plugin listening on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
