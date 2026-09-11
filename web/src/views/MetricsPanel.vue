<template>
  <div class="glass-panel p-4">
    <div class="flex items-center justify-between mb-4">
      <h3 class="text-sm font-mono text-purple">METRICS</h3>
      <div class="flex items-center gap-3">
        <span class="text-xs text-gray-600 font-mono">● LIVE</span>
        <button @click="refresh" class="text-xs text-gray-600 hover:text-purple font-mono" :disabled="loading">↻ refresh</button>
      </div>
    </div>

    <div v-if="loading" class="text-center text-gray-600 py-8 text-xs">Loading metrics...</div>

    <div v-else class="space-y-4">
      <!-- Top KPI row -->
      <div class="grid grid-cols-4 gap-3">
        <div class="glass-panel p-3 text-center">
          <div class="text-2xl font-mono" :class="kpi.agents > 0 ? 'text-neon' : 'text-gray-600'">{{ kpi.agents }}</div>
          <div class="text-xs text-gray-500 mt-1">Agents</div>
        </div>
        <div class="glass-panel p-3 text-center">
          <div class="text-2xl font-mono" :class="kpi.hosts > 0 ? 'text-purple' : 'text-gray-600'">{{ kpi.hosts }}</div>
          <div class="text-xs text-gray-500 mt-1">Hosts</div>
        </div>
        <div class="glass-panel p-3 text-center">
          <div class="text-2xl font-mono" :class="kpi.vulns > 0 ? 'text-yellow-400' : 'text-gray-600'">{{ kpi.vulns }}</div>
          <div class="text-xs text-gray-500 mt-1">Vulns</div>
        </div>
        <div class="glass-panel p-3 text-center">
          <div class="text-2xl font-mono" :class="kpi.creds > 0 ? 'text-alert' : 'text-gray-600'">{{ kpi.creds }}</div>
          <div class="text-xs text-gray-500 mt-1">Creds</div>
        </div>
      </div>

      <!-- Real counters only — no invented evasion/exploit rates -->
      <div class="grid grid-cols-3 gap-3">
        <div class="glass-panel p-3 text-center">
          <div class="text-3xl font-mono text-neon">{{ activeAgents }}</div>
          <div class="text-xs text-gray-500 mt-1">Active Agents</div>
        </div>
        <div class="glass-panel p-3 text-center">
          <div class="text-3xl font-mono text-purple">{{ kpi.lateral }}</div>
          <div class="text-xs text-gray-500 mt-1">Lateral Moves</div>
        </div>
        <div class="glass-panel p-3 text-center">
          <div class="text-3xl font-mono" :class="blueAlerts > 0 ? 'text-alert' : 'text-gray-600'">{{ blueAlerts }}</div>
          <div class="text-xs text-gray-500 mt-1">Blue Detections</div>
        </div>
      </div>

      <!-- Detection log or empty state -->
      <div class="glass-panel p-3">
        <div class="text-xs text-gray-500 font-mono mb-2">Detection Log</div>
        <div v-if="blueMetrics.length > 0" class="space-y-1 text-xs font-mono max-h-40 overflow-y-auto">
          <div v-for="(m, i) in blueMetrics" :key="i" class="flex gap-2 items-center">
            <span :class="m.detected ? 'text-alert' : 'text-neon'">{{ m.detected ? '✗' : '✓' }}</span>
            <span class="text-gray-500">[{{ m.tool || '?' }}]</span>
            <span :class="m.detected ? 'text-gray-300' : 'text-gray-500'">
              {{ m.detected ? (m.alert_type || 'DETECTED') : ('Bypassed: ' + (m.tool || 'evasion')) }}
            </span>
            <span class="text-gray-700 ml-auto text-xs">{{ formatTime(m.timestamp) }}</span>
          </div>
        </div>
        <div v-else class="text-gray-600 text-xs py-4 text-center">
          No detection events yet. Deploy agents to collect metrics.
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useMetricsStore } from '../stores/index.js'

const metricsStore = useMetricsStore()
const loading = ref(false)
const rawMetrics = ref({})
const blueMetrics = ref([])

const kpi = computed(() => ({
  agents:  rawMetrics.value.total_agents         ?? 0,
  hosts:   rawMetrics.value.total_hosts          ?? 0,
  vulns:   rawMetrics.value.total_vulns          ?? 0,
  creds:   rawMetrics.value.credentials_captured ?? 0,
  lateral: rawMetrics.value.lateral_moves        ?? 0,
}))

// only real counters — the old "Evasion Rate" was an invented formula
const activeAgents = computed(() => rawMetrics.value.active_agents ?? 0)

const blueAlerts = computed(() => blueMetrics.value.filter(m => m.detected).length)

const formatTime = (ts) => ts ? new Date(ts).toLocaleTimeString() : '--:--'

const refresh = async () => {
  loading.value = true
  await Promise.all([
    metricsStore.fetchMetrics().then(() => { rawMetrics.value = metricsStore.metrics }).catch(() => {}),
    metricsStore.fetchBlueMetrics().then(() => { blueMetrics.value = metricsStore.blueMetrics }).catch(() => {}),
  ])
  loading.value = false
}

onMounted(refresh)
</script>
