// server/provider_traffic.go：基础模式服务器剩余流量站内接口（Build41 Step 4）。
// 首页只读采集器内存结果；管理端设置读取本地已提交状态，写入与手动采集统一经采集器交接。
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
	"vpn-sub/internal/providertraffic"
)

// providerTrafficSettingsReader 提供已提交的本地设置读取；由 providertraffic.Service 实现。
type providerTrafficSettingsReader interface {
	GetSettings(ctx context.Context) (providertraffic.Settings, error)
}

// providerTrafficCollector 是接入层所需的采集器能力；写入由 Collector 统一完成提交与调度交接。
type providerTrafficCollector interface {
	Results(ctx context.Context) ([]providertraffic.Result, error)
	ResultForRevision(provider providertraffic.Provider, revision string) *providertraffic.Result
	SetEnabled(ctx context.Context, enabled bool) (providertraffic.Settings, error)
	SaveBinding(ctx context.Context, in providertraffic.SaveBindingInput) (providertraffic.Settings, error)
	ClearBinding(ctx context.Context, provider providertraffic.Provider, revision string) (providertraffic.Settings, error)
	ManualRefresh(ctx context.Context, provider providertraffic.Provider) (providertraffic.Result, error)
}

// ProviderTrafficHandler 服务器剩余流量处理器（结构体 Handler + 依赖注入）。
type ProviderTrafficHandler struct {
	settingsSvc providerTrafficSettingsReader
	collector   providerTrafficCollector
	cfg         *config.Service
}

// providerTrafficProviders 是两家固定展示顺序。
var providerTrafficProviders = [...]providertraffic.Provider{
	providertraffic.ProviderKiwiVM,
	providertraffic.ProviderVPS,
}

// RegisterProviderTrafficRoutes 注册首页只读与管理端设置路由。
// no-store 必须先于 session/admin 执行，保证 401/403 等中间件响应也带禁止缓存头。
func RegisterProviderTrafficRoutes(engine *gin.Engine, h *ProviderTrafficHandler, sessionMW, adminMW gin.HandlerFunc) {
	home := engine.Group("/api/home", noStoreMiddleware(), sessionMW)
	home.GET("/provider-traffic", h.home)

	admin := engine.Group("/api/admin/settings/provider-traffic", noStoreMiddleware(), sessionMW, adminMW)
	admin.GET("", h.getSettings)
	admin.PUT("", h.saveEnabled)
	admin.PUT("/:provider", h.saveBinding)
	admin.DELETE("/:provider", h.deleteBinding)
	admin.POST("/:provider/refresh", h.refresh)
}

// home 返回当前启用且已绑定的平台本地结果；从不触发上游请求。
func (h *ProviderTrafficHandler) home(c *gin.Context) {
	results, err := h.collector.Results(c.Request.Context())
	if err != nil {
		failProviderTrafficInternal(c, err)
		return
	}
	list := make([]providerTrafficPublicResult, 0, len(results))
	for _, result := range results {
		list = append(list, newProviderTrafficPublicResult(result))
	}
	OK(c, ListData{List: list, Total: int64(len(list))})
}

// getSettings 返回基础模式下的两家本地设置与各自最近结果；高级模式/Setup 未完成返回不可用结构。
func (h *ProviderTrafficHandler) getSettings(c *gin.Context) {
	ctx := c.Request.Context()
	available, err := h.available(ctx)
	if err != nil {
		failProviderTrafficInternal(c, err)
		return
	}
	if !available {
		OK(c, emptyProviderTrafficSettings())
		return
	}
	settings, err := h.settingsSvc.GetSettings(ctx)
	if err != nil {
		failProviderTrafficInternal(c, err)
		return
	}
	OK(c, h.settingsView(settings))
}

// saveEnabled 只保存总开关；提交后由采集器按本次变化安排首轮采集。
func (h *ProviderTrafficHandler) saveEnabled(c *gin.Context) {
	if !h.requireWritable(c) {
		return
	}
	fields, ok := parseProviderTrafficFields(c)
	if !ok {
		return
	}
	enabled, ok := providerTrafficBoolField(fields, "enabled")
	if !ok {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	settings, err := h.collector.SetEnabled(c.Request.Context(), enabled)
	if err != nil {
		failProviderTraffic(c, err)
		return
	}
	OK(c, h.settingsView(settings))
}

// saveBinding 保存单家开关与绑定；省略 credential 表示同一实例保留当前密文。
func (h *ProviderTrafficHandler) saveBinding(c *gin.Context) {
	provider, ok := h.provider(c)
	if !ok {
		return
	}
	if !h.requireWritable(c) {
		return
	}
	fields, ok := parseProviderTrafficFields(c)
	if !ok {
		return
	}
	enabled, ok := providerTrafficBoolField(fields, "enabled")
	if !ok {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	instanceID, ok := providerTrafficStringField(fields, "instance_id")
	if !ok {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	revision, ok := providerTrafficRevisionField(fields)
	if !ok {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	credential, ok := providerTrafficCredentialField(fields)
	if !ok {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	settings, err := h.collector.SaveBinding(c.Request.Context(), providertraffic.SaveBindingInput{
		Provider:   provider,
		Enabled:    enabled,
		InstanceID: instanceID,
		Revision:   revision,
		Credential: credential,
	})
	if err != nil {
		failProviderTraffic(c, err)
		return
	}
	OK(c, h.settingsView(settings))
}

// deleteBinding 按当前绑定修订解除单家绑定，并清除该修订的内存结果。
func (h *ProviderTrafficHandler) deleteBinding(c *gin.Context) {
	provider, ok := h.provider(c)
	if !ok {
		return
	}
	if !h.requireWritable(c) {
		return
	}
	fields, ok := parseProviderTrafficFields(c)
	if !ok {
		return
	}
	revision, ok := providerTrafficDeleteRevisionField(fields)
	if !ok {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	settings, err := h.collector.ClearBinding(c.Request.Context(), provider, revision)
	if err != nil {
		failProviderTraffic(c, err)
		return
	}
	OK(c, h.settingsView(settings))
}

// refresh 仅使用当前已保存绑定同步采集一次；上游失败仍作为平台结果返回。
func (h *ProviderTrafficHandler) refresh(c *gin.Context) {
	provider, ok := h.provider(c)
	if !ok {
		return
	}
	if !h.requireWritable(c) {
		return
	}
	result, err := h.collector.ManualRefresh(c.Request.Context(), provider)
	if err != nil {
		failProviderTraffic(c, err)
		return
	}
	OK(c, newProviderTrafficAdminResult(result))
}

// provider 校验路径中的固定平台枚举。
func (h *ProviderTrafficHandler) provider(c *gin.Context) (providertraffic.Provider, bool) {
	provider, ok := parseProviderTrafficProvider(c.Param("provider"))
	if !ok {
		Fail(c, http.StatusBadRequest, "未知的服务商")
		return "", false
	}
	return provider, true
}

// available 实时判断当前是否允许使用本功能：Setup 已完成且处于基础模式。
func (h *ProviderTrafficHandler) available(ctx context.Context) (bool, error) {
	configured, err := h.cfg.GetBoolStrict(ctx, config.KeyConfigured, false)
	if err != nil {
		return false, err
	}
	if !configured {
		return false, nil
	}
	advanced, err := h.cfg.GetBoolStrict(ctx, config.KeyAdvancedMode, false)
	if err != nil {
		return false, err
	}
	return !advanced, nil
}

// requireWritable 对写操作与手动采集执行实时模式校验；失败时已写入响应。
func (h *ProviderTrafficHandler) requireWritable(c *gin.Context) bool {
	available, err := h.available(c.Request.Context())
	if err != nil {
		failProviderTrafficInternal(c, err)
		return false
	}
	if !available {
		Fail(c, http.StatusForbidden, "当前模式不可用")
		return false
	}
	return true
}

// settingsView 固定按两家顺序组装设置；结果只在与当前绑定修订匹配时返回。
func (h *ProviderTrafficHandler) settingsView(settings providertraffic.Settings) providerTrafficSettingsView {
	providers := make([]providerTrafficBindingView, 0, len(providerTrafficProviders))
	for _, provider := range providerTrafficProviders {
		binding := bindingFromProviderTrafficSettings(settings, provider)
		item := providerTrafficBindingView{
			Provider:             string(provider),
			Enabled:              binding.Enabled,
			CredentialConfigured: binding.CredentialConfigured,
		}
		if binding.InstanceID != "" {
			instanceID := binding.InstanceID
			item.InstanceID = &instanceID
		}
		if binding.Revision != "" {
			revision := binding.Revision
			item.Revision = &revision
		}
		if binding.InstanceID != "" && binding.Revision != "" {
			if result := h.collector.ResultForRevision(provider, binding.Revision); result != nil {
				view := newProviderTrafficAdminResult(*result)
				item.Result = &view
			}
		}
		providers = append(providers, item)
	}
	return providerTrafficSettingsView{
		Available: true,
		Enabled:   settings.Enabled,
		Providers: providers,
	}
}

// bindingFromProviderTrafficSettings 按平台取已提交绑定；缺失时按未绑定处理。
func bindingFromProviderTrafficSettings(settings providertraffic.Settings, provider providertraffic.Provider) providertraffic.ProviderBinding {
	for _, binding := range settings.Providers {
		if binding.Provider == provider {
			return binding
		}
	}
	return providertraffic.ProviderBinding{Provider: provider}
}

// emptyProviderTrafficSettings 是模式不可用时的固定响应。
func emptyProviderTrafficSettings() providerTrafficSettingsView {
	return providerTrafficSettingsView{Providers: []providerTrafficBindingView{}}
}

// providerTrafficSettingsView 是对外 Settings 结构。
type providerTrafficSettingsView struct {
	Available bool                         `json:"available"`
	Enabled   bool                         `json:"enabled"`
	Providers []providerTrafficBindingView `json:"providers"`
}

// providerTrafficBindingView 是单家对外设置；凭据只以保存标记表示。
type providerTrafficBindingView struct {
	Provider             string                          `json:"provider"`
	Enabled              bool                            `json:"enabled"`
	InstanceID           *string                         `json:"instance_id"`
	Revision             *string                         `json:"revision"`
	CredentialConfigured bool                            `json:"credential_configured"`
	Result               *providerTrafficAdminResultView `json:"result"`
}

// providerTrafficPublicResult 是首页可见的结果字段，不包含 hint、实例、修订或凭据。
type providerTrafficPublicResult struct {
	Provider  string   `json:"provider"`
	Status    string   `json:"status"`
	Remaining *float64 `json:"remaining,omitempty"`
	Unit      string   `json:"unit,omitempty"`
	SampledAt string   `json:"sampled_at,omitempty"`
	Message   string   `json:"message,omitempty"`
}

// providerTrafficAdminResultView 是管理端结果，可在错误时附安全操作提示。
type providerTrafficAdminResultView struct {
	Provider  string   `json:"provider"`
	Status    string   `json:"status"`
	Remaining *float64 `json:"remaining,omitempty"`
	Unit      string   `json:"unit,omitempty"`
	SampledAt string   `json:"sampled_at,omitempty"`
	Message   string   `json:"message,omitempty"`
	Hint      string   `json:"hint,omitempty"`
}

// newProviderTrafficPublicResult 按状态只输出合同允许的公共字段。
func newProviderTrafficPublicResult(result providertraffic.Result) providerTrafficPublicResult {
	view := providerTrafficPublicResult{Provider: string(result.Provider), Status: string(result.Status)}
	switch result.Status {
	case providertraffic.ResultOK:
		remaining := result.Remaining
		view.Remaining = &remaining
		view.Unit = result.Unit
		view.SampledAt = result.SampledAt.UTC().Format(time.RFC3339)
	case providertraffic.ResultError:
		view.Message = result.Message
	}
	return view
}

// newProviderTrafficAdminResult 在公共字段基础上仅对错误结果追加 hint。
func newProviderTrafficAdminResult(result providertraffic.Result) providerTrafficAdminResultView {
	public := newProviderTrafficPublicResult(result)
	view := providerTrafficAdminResultView{
		Provider:  public.Provider,
		Status:    public.Status,
		Remaining: public.Remaining,
		Unit:      public.Unit,
		SampledAt: public.SampledAt,
		Message:   public.Message,
	}
	if result.Status == providertraffic.ResultError {
		view.Hint = result.Hint
	}
	return view
}

// parseProviderTrafficFields 要求完整请求体只有一个 JSON 对象。
func parseProviderTrafficFields(c *gin.Context) (map[string]json.RawMessage, bool) {
	var fields map[string]json.RawMessage
	decoder := json.NewDecoder(c.Request.Body)
	err := decoder.Decode(&fields)
	var whitespaceOnly bool
	if err == nil {
		whitespaceOnly, err = providerTrafficJSONTail(io.MultiReader(decoder.Buffered(), c.Request.Body))
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		Fail(c, http.StatusRequestEntityTooLarge, "请求体过大")
		return nil, false
	}
	if err != nil || fields == nil || !whitespaceOnly {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return nil, false
	}
	return fields, true
}

// providerTrafficJSONTail 以固定缓冲区检查 JSON 空白，读到 EOF 或现有请求体上限。
func providerTrafficJSONTail(tail io.Reader) (bool, error) {
	var buffer [4096]byte
	whitespaceOnly := true
	for {
		n, err := tail.Read(buffer[:])
		if whitespaceOnly && len(bytes.Trim(buffer[:n], " \t\r\n")) != 0 {
			whitespaceOnly = false
		}
		if errors.Is(err, io.EOF) {
			return whitespaceOnly, nil
		}
		if err != nil {
			return false, err
		}
	}
}

// providerTrafficBoolField 要求字段存在且为 JSON 布尔值；null 视为无效。
func providerTrafficBoolField(fields map[string]json.RawMessage, key string) (bool, bool) {
	raw, ok := fields[key]
	if !ok || isProviderTrafficJSONNull(raw) {
		return false, false
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, false
	}
	return value, true
}

// providerTrafficStringField 要求字符串字段存在且不是 null；空串交给业务层校验。
func providerTrafficStringField(fields map[string]json.RawMessage, key string) (string, bool) {
	raw, ok := fields[key]
	if !ok || isProviderTrafficJSONNull(raw) {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

// providerTrafficRevisionField 要求 revision 字段显式存在；null 映射为首次绑定。
func providerTrafficRevisionField(fields map[string]json.RawMessage) (*string, bool) {
	raw, ok := fields["revision"]
	if !ok {
		return nil, false
	}
	if isProviderTrafficJSONNull(raw) {
		return nil, true
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false
	}
	return &value, true
}

// providerTrafficDeleteRevisionField 要求解除绑定的 revision 为非 null 字符串。
func providerTrafficDeleteRevisionField(fields map[string]json.RawMessage) (string, bool) {
	raw, ok := fields["revision"]
	if !ok || isProviderTrafficJSONNull(raw) {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

// providerTrafficCredentialField 区分省略、null 与字符串；空串/纯空白由业务层拒绝。
func providerTrafficCredentialField(fields map[string]json.RawMessage) (providertraffic.CredentialInput, bool) {
	raw, ok := fields["credential"]
	if !ok {
		return providertraffic.CredentialInput{Omitted: true}, true
	}
	input := providertraffic.CredentialInput{}
	if isProviderTrafficJSONNull(raw) {
		input.Null = true
		return input, true
	}
	if err := json.Unmarshal(raw, &input.Value); err != nil {
		return providertraffic.CredentialInput{}, false
	}
	return input, true
}

func isProviderTrafficJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func parseProviderTrafficProvider(raw string) (providertraffic.Provider, bool) {
	switch providertraffic.Provider(raw) {
	case providertraffic.ProviderKiwiVM:
		return providertraffic.ProviderKiwiVM, true
	case providertraffic.ProviderVPS:
		return providertraffic.ProviderVPS, true
	default:
		return "", false
	}
}

// failProviderTraffic 将业务错误映射为冻结合同的站内状态码；未知错误按 500 通用正文。
func failProviderTraffic(c *gin.Context, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		Fail(c, http.StatusRequestEntityTooLarge, "请求体过大")
		return
	}
	switch {
	case errors.Is(err, providertraffic.ErrUnknownProvider):
		Fail(c, http.StatusBadRequest, "未知的服务商")
	case errors.Is(err, providertraffic.ErrInvalidInstanceID),
		errors.Is(err, providertraffic.ErrInvalidCredential),
		errors.Is(err, providertraffic.ErrCredentialRequired):
		Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, providertraffic.ErrRevisionConflict):
		Fail(c, http.StatusConflict, "绑定修订冲突")
	case errors.Is(err, providertraffic.ErrCollectorBusy):
		Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, providertraffic.ErrNotReady):
		Fail(c, http.StatusConflict, "当前绑定未启用")
	case errors.Is(err, providertraffic.ErrDiscarded):
		Fail(c, http.StatusConflict, "本轮采集结果已丢弃")
	case errors.Is(err, providertraffic.ErrModeUnavailable):
		Fail(c, http.StatusForbidden, "当前模式不可用")
	default:
		failProviderTrafficInternal(c, err)
	}
}

// failProviderTrafficInternal 对本地读写或生命周期故障返回固定通用正文。
func failProviderTrafficInternal(c *gin.Context, err error) {
	FailSanitized(c, http.StatusInternalServerError, "服务器内部错误", err)
}

// isProviderTrafficAPIPath 精确识别本模块路由，用于全局 413 短路前补 no-store。
func isProviderTrafficAPIPath(fullPath string) bool {
	switch fullPath {
	case "/api/home/provider-traffic",
		"/api/admin/settings/provider-traffic",
		"/api/admin/settings/provider-traffic/:provider",
		"/api/admin/settings/provider-traffic/:provider/refresh":
		return true
	default:
		return false
	}
}
