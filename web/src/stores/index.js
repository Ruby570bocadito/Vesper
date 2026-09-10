import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

const API_BASE = '/api'

export const useAgentStore = defineStore('agents', () => {
  const agents = ref([])
  const loading = ref(false)
  const error = ref(null)

  const activeAgents = computed(() => agents.value.filter(a => a.status === 'online' || a.status === 'active'))
  const deadAgents = computed(() => agents.value.filter(a => a.status === 'dead'))
  const agentCount = computed(() => agents.value.length)

  async function fetchAgents(campaignId = null) {
    loading.value = true
    error.value = null
    try {
      const params = campaignId ? `?campaign_id=${campaignId}` : ''
      const res = await fetch(`${API_BASE}/agents${params}`)
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      agents.value = await res.json()
    } catch (e) {
      error.value = e.message
    } finally {
      loading.value = false
    }
  }

  async function getAgent(id) {
    try {
      const res = await fetch(`${API_BASE}/agents/${id}`)
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      return await res.json()
    } catch (e) {
      error.value = e.message
      return null
    }
  }

  async function killAgent(id, reason = '') {
    try {
      const res = await fetch(`${API_BASE}/agents/${id}/kill`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ reason }),
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data = await res.json()
      if (data.success) {
        agents.value = agents.value.filter(a => a.id !== id)
      }
      return data
    } catch (e) {
      error.value = e.message
      return null
    }
  }

  return { agents, loading, error, activeAgents, deadAgents, agentCount, fetchAgents, getAgent, killAgent }
})

export const useCampaignStore = defineStore('campaigns', () => {
  const campaigns = ref([])
  const activeCampaign = ref(null)
  const loading = ref(false)

  async function fetchCampaigns() {
    loading.value = true
    try {
      const res = await fetch(`${API_BASE}/campaigns`)
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      campaigns.value = await res.json()
      // auto-select the running campaign so the header reflects reality
      if (!activeCampaign.value && campaigns.value.length) {
        activeCampaign.value =
          campaigns.value.find(c => c.status === 'running') || campaigns.value[0]
      }
    } catch (e) {
      console.error('Failed to fetch campaigns:', e)
    } finally {
      loading.value = false
    }
  }

  async function createCampaign(name, targetScope, goal, profile = 'balanced', autoApprove = false) {
    const res = await fetch(`${API_BASE}/campaigns`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, target_scope: targetScope, goal, profile, auto_approve: autoApprove }),
    })
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    const campaign = await res.json()
    campaigns.value.push(campaign)
    return campaign
  }

  async function getCampaign(id) {
    const res = await fetch(`${API_BASE}/campaigns/${id}`)
    if (!res.ok) throw new Error(`HTTP ${res.status}`)
    activeCampaign.value = await res.json()
    return activeCampaign.value
  }

  async function pauseCampaign(id) {
    const res = await fetch(`${API_BASE}/campaigns/${id}/pause`, { method: 'POST' })
    return res.ok
  }

  async function resumeCampaign(id) {
    const res = await fetch(`${API_BASE}/campaigns/${id}/resume`, { method: 'POST' })
    return res.ok
  }

  return { campaigns, activeCampaign, loading, fetchCampaigns, createCampaign, getCampaign, pauseCampaign, resumeCampaign }
})

export const useReconStore = defineStore('recon', () => {
  const hosts = ref([])
  const services = ref([])
  const vulnerabilities = ref([])
  const loading = ref(false)

  async function fetchHosts(campaignId = null) {
    loading.value = true
    try {
      const params = campaignId ? `?campaign_id=${campaignId}` : ''
      const res = await fetch(`${API_BASE}/hosts${params}`)
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      hosts.value = await res.json()
    } finally {
      loading.value = false
    }
  }

  async function fetchServices(campaignId = null) {
    const params = campaignId ? `?campaign_id=${campaignId}` : ''
    const res = await fetch(`${API_BASE}/services${params}`)
    if (res.ok) services.value = await res.json()
  }

  async function fetchVulnerabilities(campaignId = null) {
    const params = campaignId ? `?campaign_id=${campaignId}` : ''
    const res = await fetch(`${API_BASE}/vulnerabilities${params}`)
    if (res.ok) vulnerabilities.value = await res.json()
  }

  async function scanHost(target, mode = 'basic') {
    const res = await fetch(`${API_BASE}/recon/scan`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ target, mode }),
    })
    return res.ok ? await res.json() : null
  }

  return { hosts, services, vulnerabilities, loading, fetchHosts, fetchServices, fetchVulnerabilities, scanHost }
})

export const useAIStore = defineStore('ai', () => {
  const messages = ref([])
  const suggestions = ref([])
  const loading = ref(false)

  async function chat(prompt) {
    messages.value.push({ role: 'user', content: prompt })
    loading.value = true
    try {
      const res = await fetch(`${API_BASE}/ai/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ prompt }),
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const data = await res.json()
      messages.value.push({ role: 'ai', content: data.response })
      return data
    } catch (e) {
      messages.value.push({ role: 'ai', content: `[Error] ${e.message}` })
    } finally {
      loading.value = false
    }
  }

  async function getSuggestions(campaignId = null) {
    loading.value = true
    try {
      const params = campaignId ? `?campaign_id=${campaignId}` : ''
      const res = await fetch(`${API_BASE}/decisions${params}`)
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      suggestions.value = await res.json()
    } finally {
      loading.value = false
    }
  }

  async function approveDecision(id) {
    const res = await fetch(`${API_BASE}/decisions/${id}/approve`, { method: 'POST' })
    if (res.ok) {
      const idx = suggestions.value.findIndex(d => d.id === id)
      if (idx >= 0) suggestions.value[idx].approved = true
    }
    return res.ok
  }

  async function rejectDecision(id) {
    const res = await fetch(`${API_BASE}/decisions/${id}/reject`, { method: 'POST' })
    if (res.ok) {
      const idx = suggestions.value.findIndex(d => d.id === id)
      if (idx >= 0) suggestions.value[idx].approved = false
    }
    return res.ok
  }

  return { messages, suggestions, loading, chat, getSuggestions, approveDecision, rejectDecision }
})

export const useMetricsStore = defineStore('metrics', () => {
  const metrics = ref({})
  const blueMetrics = ref([])

  async function fetchMetrics(campaignId = null) {
    const params = campaignId ? `?campaign_id=${campaignId}` : ''
    try {
      const res = await fetch(`${API_BASE}/metrics${params}`)
      if (res.ok) metrics.value = await res.json()
    } catch (e) {
      console.error('Failed to fetch metrics:', e)
    }
  }

  async function fetchBlueMetrics(campaignId = null) {
    const params = campaignId ? `?campaign_id=${campaignId}` : ''
    try {
      const res = await fetch(`${API_BASE}/blue/metrics${params}`)
      if (res.ok) blueMetrics.value = await res.json()
    } catch (e) {
      console.error('Failed to fetch blue metrics:', e)
    }
  }

  return { metrics, blueMetrics, fetchMetrics, fetchBlueMetrics }
})

export const useEventStore = defineStore('events', () => {
  const events = ref([])
  const ws = ref(null)
  const connected = ref(false)

  function connect(campaignId = null) {
    if (ws.value) return
    const protocol = location.protocol === 'https:' ? 'wss' : 'ws'
    const url = campaignId
      ? `${protocol}://${location.host}/ws?campaign_id=${campaignId}`
      : `${protocol}://${location.host}/ws`

    ws.value = new WebSocket(url)
    ws.value.onopen = () => { connected.value = true }
    ws.value.onclose = () => { connected.value = false; ws.value = null }
    ws.value.onerror = () => { connected.value = false }
    ws.value.onmessage = (msg) => {
      try {
        const event = JSON.parse(msg.data)
        events.value.unshift(event)
        if (events.value.length > 500) events.value.pop()
      } catch {}
    }
  }

  function disconnect() {
    if (ws.value) {
      ws.value.close()
      ws.value = null
    }
    connected.value = false
  }

  return { events, connected, connect, disconnect }
})
