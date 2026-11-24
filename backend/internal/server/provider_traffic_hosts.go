package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/providertraffic"
)

type providerTrafficHostView struct {
	ID                   string                          `json:"id"`
	Provider             string                          `json:"provider"`
	Name                 string                          `json:"name"`
	Enabled              bool                            `json:"enabled"`
	InstanceID           string                          `json:"instance_id"`
	Revision             string                          `json:"revision"`
	HalfRemaining        bool                            `json:"half_remaining"`
	CredentialConfigured bool                            `json:"credential_configured"`
	Result               *providerTrafficAdminResultView `json:"result"`
}

type providerTrafficAccountView struct {
	Configured bool   `json:"configured"`
	Revision   string `json:"revision"`
}

func (h *ProviderTrafficHandler) saveHost(c *gin.Context) {
	provider, ok := h.provider(c)
	if !ok || !h.requireWritable(c) {
		return
	}
	fields, ok := parseProviderTrafficFields(c)
	if !ok {
		return
	}
	name, nameOK := providerTrafficStringField(fields, "name")
	instance, instanceOK := providerTrafficStringField(fields, "instance_id")
	enabled, enabledOK := providerTrafficBoolField(fields, "enabled")
	half, halfOK := providerTrafficBoolField(fields, "half_remaining")
	revision, revisionOK := providerTrafficRevisionField(fields)
	credential, credentialOK := providerTrafficCredentialField(fields)
	if !nameOK || !instanceOK || !enabledOK || !halfOK || !revisionOK || !credentialOK {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	settings, err := h.collector.SaveHost(c.Request.Context(), providertraffic.SaveHostInput{
		Provider: provider, ID: c.Param("host"), Name: name, InstanceID: instance,
		Enabled: enabled, HalfRemaining: half, Revision: revision, Credential: credential,
	})
	if err != nil {
		failProviderTraffic(c, err)
		return
	}
	OK(c, h.settingsView(settings))
}

func (h *ProviderTrafficHandler) deleteHost(c *gin.Context) {
	provider, ok := h.provider(c)
	if !ok || !h.requireWritable(c) {
		return
	}
	revision, ok := requireHostRevision(c)
	if !ok {
		return
	}
	settings, err := h.collector.ClearHost(c.Request.Context(), provider, c.Param("host"), revision)
	if err != nil {
		failProviderTraffic(c, err)
		return
	}
	OK(c, h.settingsView(settings))
}

func (h *ProviderTrafficHandler) refreshHost(c *gin.Context) {
	provider, ok := h.provider(c)
	if !ok || !h.requireWritable(c) {
		return
	}
	revision, ok := requireHostRevision(c)
	if !ok {
		return
	}
	result, err := h.collector.RefreshHost(c.Request.Context(), provider, c.Param("host"), revision)
	if err != nil {
		failProviderTraffic(c, err)
		return
	}
	OK(c, newProviderTrafficAdminResult(result))
}

func requireHostRevision(c *gin.Context) (string, bool) {
	fields, ok := parseProviderTrafficFields(c)
	if !ok {
		return "", false
	}
	revision, ok := providerTrafficStringField(fields, "revision")
	if !ok || strings.TrimSpace(revision) == "" {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return "", false
	}
	return revision, true
}

func (h *ProviderTrafficHandler) saveVPSAccount(c *gin.Context) {
	if !h.requireWritable(c) {
		return
	}
	fields, ok := parseProviderTrafficFields(c)
	if !ok {
		return
	}
	revision, revisionOK := providerTrafficStringField(fields, "revision")
	var credential string
	if c.Request.Method == http.MethodDelete {
		if strings.TrimSpace(revision) == "" {
			revisionOK = false
		}
	} else {
		var credentialOK bool
		credential, credentialOK = providerTrafficStringField(fields, "credential")
		credential = strings.TrimSpace(credential)
		if !credentialOK || credential == "" {
			Fail(c, http.StatusBadRequest, "凭据必须为非空字符串")
			return
		}
	}
	if !revisionOK {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	settings, err := h.collector.SaveVPSAccount(c.Request.Context(), revision, credential)
	if err != nil {
		failProviderTraffic(c, err)
		return
	}
	OK(c, h.settingsView(settings))
}
