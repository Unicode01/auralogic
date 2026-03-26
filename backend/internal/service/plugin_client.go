package service

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/pb"
	"auralogic/internal/pluginipc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// PluginClient gRPC 插件客户端
type PluginClient struct {
	ID          uint
	Name        string
	Address     string
	transport   config.PluginGRPCTransportConfig
	mu          sync.RWMutex
	conn        *grpc.ClientConn
	client      pb.PluginServiceClient
	lastHealthy time.Time
	failCount   int
	connected   bool
}

// PluginHealth 插件健康状态
type PluginHealth struct {
	Healthy  bool
	Version  string
	Metadata map[string]string
}

// NewPluginClient 创建插件客户端
func NewPluginClient(id uint, name, address string, transport ...config.PluginGRPCTransportConfig) *PluginClient {
	grpcTransport := config.PluginGRPCTransportConfig{
		Mode: "insecure_local",
	}
	if len(transport) > 0 {
		grpcTransport = transport[0]
	}
	grpcTransport.Mode = normalizeGRPCTransportMode(grpcTransport.Mode)
	return &PluginClient{
		ID:        id,
		Name:      name,
		Address:   address,
		transport: grpcTransport,
	}
}

// Connect 建立连接
func (c *PluginClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected && c.conn != nil && c.client != nil {
		return nil
	}

	dialOptions, err := c.buildDialOptions()
	if err != nil {
		return fmt.Errorf("resolve transport for plugin %s (%s): %w", c.Name, c.Address, err)
	}
	dialOptions = append(dialOptions, grpc.WithBlock())

	conn, err := grpc.DialContext(ctx, c.Address, dialOptions...)
	if err != nil {
		return fmt.Errorf("connect plugin %s (%s): %w", c.Name, c.Address, err)
	}

	client := pb.NewPluginServiceClient(conn)
	healthResp, err := client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("initial health check failed for plugin %s: %w", c.Name, err)
	}
	if !healthResp.GetHealthy() {
		_ = conn.Close()
		return fmt.Errorf("plugin %s reported unhealthy on connect", c.Name)
	}

	c.conn = conn
	c.client = client
	c.connected = true
	c.lastHealthy = time.Now()
	c.failCount = 0
	return nil
}

func (c *PluginClient) buildDialOptions() ([]grpc.DialOption, error) {
	mode := normalizeGRPCTransportMode(c.transport.Mode)
	switch mode {
	case "tls":
		creds, err := c.buildTLSCredentials()
		if err != nil {
			return nil, err
		}
		return []grpc.DialOption{grpc.WithTransportCredentials(creds)}, nil
	case "insecure":
		return []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}, nil
	case "insecure_local":
		if !isLocalPluginAddress(c.Address) {
			return nil, fmt.Errorf("insecure_local transport only allows loopback or unix endpoints")
		}
		return []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}, nil
	default:
		return nil, fmt.Errorf("unsupported grpc transport mode %q", mode)
	}
}

func (c *PluginClient) buildTLSCredentials() (credentials.TransportCredentials, error) {
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: c.transport.InsecureSkipVerify,
	}
	if serverName := strings.TrimSpace(c.transport.ServerName); serverName != "" {
		tlsConfig.ServerName = serverName
	}

	caFile := normalizeOptionalFilePath(c.transport.CAFile)
	if caFile != "" {
		caRaw, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read grpc ca file failed: %w", err)
		}

		roots, err := x509.SystemCertPool()
		if err != nil || roots == nil {
			roots = x509.NewCertPool()
		}
		if ok := roots.AppendCertsFromPEM(caRaw); !ok {
			return nil, fmt.Errorf("parse grpc ca file failed")
		}
		tlsConfig.RootCAs = roots
	}

	certFile := normalizeOptionalFilePath(c.transport.CertFile)
	keyFile := normalizeOptionalFilePath(c.transport.KeyFile)
	if (certFile == "") != (keyFile == "") {
		return nil, fmt.Errorf("grpc cert_file and key_file must be provided together")
	}
	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("load grpc client certificate failed: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return credentials.NewTLS(tlsConfig), nil
}

func normalizeGRPCTransportMode(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	switch normalized {
	case "tls", "insecure", "insecure_local":
		return normalized
	default:
		return "insecure_local"
	}
}

func normalizeOptionalFilePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	normalized := filepath.Clean(filepath.FromSlash(trimmed))
	if normalized == "." {
		return ""
	}
	return normalized
}

func isLocalPluginAddress(address string) bool {
	target := strings.TrimSpace(address)
	if target == "" {
		return false
	}

	lower := strings.ToLower(target)
	if strings.HasPrefix(lower, "unix://") || strings.HasPrefix(lower, "unix:") {
		return true
	}
	if strings.HasPrefix(target, "/") || strings.HasPrefix(target, "\\\\") {
		return true
	}

	for _, prefix := range []string{"dns:///", "passthrough:///", "ipv4:///", "ipv6:///"} {
		if strings.HasPrefix(strings.ToLower(target), prefix) {
			target = strings.TrimSpace(target[len(prefix):])
			break
		}
	}

	if strings.Contains(target, "://") {
		if parsed, err := url.Parse(target); err == nil {
			target = parsed.Host
		}
	}

	host := target
	if parsedHost, _, err := net.SplitHostPort(target); err == nil {
		host = parsedHost
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		return true
	}

	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Close 关闭连接
func (c *PluginClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false
	c.client = nil
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// Execute 执行插件
func (c *PluginClient) Execute(ctx context.Context, action string, params map[string]string, execCtx *ExecutionContext) (*ExecutionResult, error) {
	c.mu.RLock()
	connected := c.connected
	client := c.client
	c.mu.RUnlock()

	if !connected || client == nil {
		return nil, fmt.Errorf("plugin %s not connected", c.Name)
	}

	if params == nil {
		params = make(map[string]string)
	}

	req := &pb.ExecuteRequest{
		Action: action,
		Params: params,
	}
	if execCtx != nil {
		req.Context = toPBExecutionContext(execCtx)
	}

	resp, err := client.Execute(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("execute plugin %s action %s: %w", c.Name, action, err)
	}

	result := &ExecutionResult{
		Success:  resp.GetSuccess(),
		Data:     parseExecuteData(resp.GetData()),
		Error:    strings.TrimSpace(resp.GetError()),
		Metadata: resp.GetMetadata(),
	}

	c.mu.Lock()
	c.lastHealthy = time.Now()
	c.failCount = 0
	c.mu.Unlock()

	return result, nil
}

// ExecuteStream 流式执行插件
func (c *PluginClient) ExecuteStream(ctx context.Context, action string, params map[string]string, execCtx *ExecutionContext, emit ExecutionStreamEmitter) (*ExecutionResult, error) {
	c.mu.RLock()
	connected := c.connected
	client := c.client
	c.mu.RUnlock()

	if !connected || client == nil {
		return nil, fmt.Errorf("plugin %s not connected", c.Name)
	}

	if params == nil {
		params = make(map[string]string)
	}

	req := &pb.ExecuteRequest{
		Action: action,
		Params: params,
	}
	if execCtx != nil {
		req.Context = toPBExecutionContext(execCtx)
	}

	stream, err := client.ExecuteStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("execute stream plugin %s action %s: %w", c.Name, action, err)
	}

	aggregator := newExecutionStreamAggregator()
	index := 0
	for {
		resp, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			return aggregator.FinalResult(), fmt.Errorf("execute stream plugin %s action %s: %w", c.Name, action, recvErr)
		}

		chunk := &ExecutionStreamChunk{
			Index:    index,
			Success:  resp.GetSuccess(),
			Data:     parseExecuteData(resp.GetData()),
			Error:    strings.TrimSpace(resp.GetError()),
			Metadata: resp.GetMetadata(),
			IsFinal:  resp.GetIsFinal(),
		}
		aggregator.Add(chunk)
		if emit != nil {
			if emitErr := emit(cloneExecutionStreamChunk(chunk)); emitErr != nil {
				return aggregator.FinalResult(), emitErr
			}
		}
		index++
	}

	if !aggregator.HasChunks() {
		return nil, fmt.Errorf("execute stream plugin %s action %s: empty stream response", c.Name, action)
	}
	if syntheticFinal := aggregator.SyntheticFinalChunk(); syntheticFinal != nil && emit != nil {
		if emitErr := emit(syntheticFinal); emitErr != nil {
			return aggregator.FinalResult(), emitErr
		}
	}

	c.mu.Lock()
	c.lastHealthy = time.Now()
	c.failCount = 0
	c.mu.Unlock()

	return aggregator.FinalResult(), nil
}

func toPBExecutionContext(execCtx *ExecutionContext) *pb.ExecutionContext {
	ctx := &pb.ExecutionContext{
		SessionId: execCtx.SessionID,
		Metadata:  execCtx.Metadata,
	}
	if ctx.Metadata == nil {
		ctx.Metadata = make(map[string]string)
	}
	if execCtx.UserID != nil {
		ctx.UserId = uint64(*execCtx.UserID)
	}
	if execCtx.OrderID != nil {
		ctx.OrderId = uint64(*execCtx.OrderID)
	}
	return ctx
}

func parseExecuteData(data string) map[string]interface{} {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return nil
	}

	var decoded interface{}
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return map[string]interface{}{"raw": data}
	}

	if obj, ok := decoded.(map[string]interface{}); ok {
		return obj
	}
	return map[string]interface{}{"value": decoded}
}

// ExecutionContext 执行上下文
type ExecutionContext struct {
	OperatorUserID *uint
	UserID         *uint
	OrderID        *uint
	SessionID      string
	Metadata       map[string]string
	Webhook        *pluginipc.WebhookRequest
	RequestContext context.Context
	RequestCache   *ExecutionRequestCache
}

// ExecutionResult 执行结果
type ExecutionResult struct {
	TaskID         string
	Success        bool
	Data           map[string]interface{}
	Storage        map[string]string
	StorageChanged bool
	Error          string
	Metadata       map[string]string
}

func executionRequestContext(execCtx *ExecutionContext) context.Context {
	if execCtx != nil && execCtx.RequestContext != nil {
		return execCtx.RequestContext
	}
	return context.Background()
}

// HealthCheck 健康检查
func (c *PluginClient) HealthCheck(ctx context.Context) (*PluginHealth, error) {
	c.mu.RLock()
	connected := c.connected
	client := c.client
	c.mu.RUnlock()

	if !connected || client == nil {
		return nil, fmt.Errorf("plugin %s not connected", c.Name)
	}

	resp, err := client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		return nil, fmt.Errorf("health check for plugin %s: %w", c.Name, err)
	}
	if !resp.GetHealthy() {
		return nil, fmt.Errorf("plugin %s reported unhealthy", c.Name)
	}

	health := &PluginHealth{
		Healthy:  resp.GetHealthy(),
		Version:  resp.GetVersion(),
		Metadata: resp.GetMetadata(),
	}

	c.mu.Lock()
	c.lastHealthy = time.Now()
	c.failCount = 0
	c.mu.Unlock()

	return health, nil
}

// GetStatus 获取状态
func (c *PluginClient) GetStatus() (healthy bool, lastHealthy time.Time, failCount int) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && c.failCount < 3, c.lastHealthy, c.failCount
}

// RecordFailure 记录失败
func (c *PluginClient) RecordFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failCount++
}
