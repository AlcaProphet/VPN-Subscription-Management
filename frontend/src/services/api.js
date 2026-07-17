import axios from 'axios'

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 15000
})

// Request interceptor — attach JWT from localStorage
api.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('jwt')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error) => Promise.reject(error)
)

// Response interceptor — 401 → auto logout
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response && error.response.status === 401) {
      localStorage.removeItem('jwt')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

// ============================================================================
// Public API (no auth required)
// ============================================================================
export const publicApi = {
  getSystemStatus() {
    return api.get('/system/status')
  },
  getPlatforms() {
    return api.get('/platforms')
  },
  getRules() {
    return api.get('/rules')
  },
  getRuleDownloadUrl(ruleId, token) {
    return `/api/v1/rules/${ruleId}/download?token=${encodeURIComponent(token)}`
  }
}

// ============================================================================
// Auth API
// ============================================================================
export const authApi = {
  getMe() {
    return api.get('/auth/me')
  }
}

// ============================================================================
// User API (JWT required)
// ============================================================================
export const userApi = {
  getUserPlatforms() {
    return api.get('/user/platforms')
  },
  getUpdateTime() {
    return api.get('/user/update-time')
  },
  refreshToken(platform, type) {
    return api.post('/user/refresh-token', { platform, type })
  }
}

// ============================================================================
// Download helpers
// ============================================================================
export const downloadApi = {
  downloadUrl(platform, type) {
    return `/api/v1/subscriptions/${platform}/download?type=${type}`
  },
  downloadPreviewUrl(platform, type) {
    return `/api/v1/subscriptions/${platform}/download/preview?type=${type}`
  },
  downloadByTokenUrl(platform, token) {
    return `/api/v1/subscriptions/${platform}/download-token?token=${encodeURIComponent(token)}`
  },
  shareDownloadUrl(id, token) {
    return `/api/v1/share/${id}/download?token=${encodeURIComponent(token)}`
  }
}

// ============================================================================
// Admin API (JWT + admin role required)
// ============================================================================
export const adminApi = {
  // Users
  users: {
    list() {
      return api.get('/admin/users')
    },
    get(id) {
      return api.get(`/admin/users/${id}`)
    },
    update(id, data) {
      return api.put(`/admin/users/${id}`, data)
    },
    delete(id) {
      return api.delete(`/admin/users/${id}`)
    },
    revokeTokens(id) {
      return api.post(`/admin/users/${id}/revoke-tokens`)
    },
    uploadCustomSub(id, platform, file) {
      const fd = new FormData()
      fd.append('file', file)
      return api.post(`/admin/users/${id}/custom-subscription?platform=${encodeURIComponent(platform)}`, fd)
    },
    uploadCustomSubVersion(id, formData) {
      return api.post(`/admin/users/${id}/custom-subscription/versions`, formData)
    },
    createCustomSubVersionFromText(id, content) {
      return api.post(`/admin/users/${id}/custom-subscription/versions`, { content }, {
        headers: { 'Content-Type': 'application/json' }
      })
    },
    deleteCustomSub(id, platform) {
      return api.delete(`/admin/users/${id}/custom-subscription?platform=${encodeURIComponent(platform)}`)
    },
    getCustomVersion(id, platform, versionId) {
      return api.get(`/admin/users/${id}/custom-subscription/versions/${versionId}?platform=${encodeURIComponent(platform)}`)
    },
    switchCustomVersion(id, platform, versionId) {
      return api.put(`/admin/users/${id}/custom-subscription/versions/${versionId}/current?platform=${encodeURIComponent(platform)}`)
    },
    deleteCustomVersion(id, platform, versionId) {
      return api.delete(`/admin/users/${id}/custom-subscription/versions/${versionId}?platform=${encodeURIComponent(platform)}`)
    },
    refreshCustomSubToken(id, platform) {
      return api.post(`/admin/users/${id}/custom-subscription/refresh-token?platform=${encodeURIComponent(platform)}`)
    }
  },

  // Subscriptions
  subscriptions: {
    list() {
      return api.get('/admin/subscriptions')
    },
    create(data) {
      return api.post('/admin/subscriptions', data)
    },
    get(id) {
      return api.get(`/admin/subscriptions/${id}`)
    },
    update(id, data) {
      return api.put(`/admin/subscriptions/${id}`, data)
    },
    delete(id) {
      return api.delete(`/admin/subscriptions/${id}`)
    },
    uploadVersion(id, formData) {
      return api.post(`/admin/subscriptions/${id}/versions`, formData)
    },
    createVersionFromText(id, content) {
      return api.post(`/admin/subscriptions/${id}/versions`, { content }, {
        headers: { 'Content-Type': 'application/json' }
      })
    },
    switchVersion(id, versionId) {
      return api.put(`/admin/subscriptions/${id}/versions/${versionId}/current`)
    },
    getVersion(id, versionId) {
      return api.get(`/admin/subscriptions/${id}/versions/${versionId}`)
    },
    deleteVersion(id, versionId) {
      return api.delete(`/admin/subscriptions/${id}/versions/${versionId}`)
    }
  },

  // Share subscriptions
  shares: {
    list() {
      return api.get('/admin/shares')
    },
    create(data) {
      return api.post('/admin/shares', data)
    },
    get(id) {
      return api.get(`/admin/shares/${id}`)
    },
    update(id, data) {
      return api.put(`/admin/shares/${id}`, data)
    },
    delete(id) {
      return api.delete(`/admin/shares/${id}`)
    },
    uploadVersion(id, formData) {
      return api.post(`/admin/shares/${id}/versions`, formData)
    },
    createVersionFromText(id, content) {
      return api.post(`/admin/shares/${id}/versions`, { content }, {
        headers: { 'Content-Type': 'application/json' }
      })
    },
    switchVersion(id, versionId) {
      return api.put(`/admin/shares/${id}/versions/${versionId}/current`)
    },
    getVersion(id, versionId) {
      return api.get(`/admin/shares/${id}/versions/${versionId}`)
    },
    deleteVersion(id, versionId) {
      return api.delete(`/admin/shares/${id}/versions/${versionId}`)
    },
    refreshToken(id) {
      return api.post(`/admin/shares/${id}/refresh-token`)
    },
    revokeToken(id) {
      return api.delete(`/admin/shares/${id}/token`)
    }
  },

  // Platforms
  platforms: {
    list() {
      return api.get('/admin/platforms')
    },
    create(data) {
      return api.post('/admin/platforms', data)
    },
    get(id) {
      return api.get(`/admin/platforms/${id}`)
    },
    update(id, data) {
      return api.put(`/admin/platforms/${id}`, data)
    },
    delete(id) {
      return api.delete(`/admin/platforms/${id}`)
    }
  },

  // Rules
  rules: {
    list() {
      return api.get('/admin/rules')
    },
    create(data) {
      return api.post('/admin/rules', data)
    },
    get(id) {
      return api.get(`/admin/rules/${id}`)
    },
    update(id, data) {
      return api.put(`/admin/rules/${id}`, data)
    },
    delete(id) {
      return api.delete(`/admin/rules/${id}`)
    },
    uploadVersion(id, formData) {
      return api.post(`/admin/rules/${id}/versions`, formData)
    },
    createVersionFromText(id, content) {
      return api.post(`/admin/rules/${id}/versions`, { content }, {
        headers: { 'Content-Type': 'application/json' }
      })
    },
    switchVersion(id, versionId) {
      return api.put(`/admin/rules/${id}/versions/${versionId}/current`)
    },
    getVersion(id, versionId) {
      return api.get(`/admin/rules/${id}/versions/${versionId}`)
    },
    deleteVersion(id, versionId) {
      return api.delete(`/admin/rules/${id}/versions/${versionId}`)
    },
    refreshToken(id) {
      return api.post(`/admin/rules/${id}/refresh-token`)
    }
  },

  // System
  system: {
    getOIDCConfig() {
      return api.get('/admin/oidc-config')
    },
    testOIDC(data) {
      return api.post('/admin/test-oidc', data)
    },
    configure(data) {
      return api.post('/admin/system/configure', data)
    },
    switchProvider(data) {
      return api.post('/admin/system/switch-provider', data)
    },
    getRateLimit() {
      return api.get('/admin/system/rate-limit')
    },
    updateRateLimit(data) {
      return api.put('/admin/system/rate-limit', data)
    }
  },

  // Logs
  logs: {
    getLogs(date) {
      return api.get('/admin/logs', { params: { date } })
    }
  }
}

export default api
