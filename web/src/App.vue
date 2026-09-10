<template>
  <div class="flex flex-col h-screen scanlines">
    <Header />
    <Tabs :active="activeTab" @switch="activeTab = $event" />
    <main class="flex-1 overflow-auto p-3">
      <Dashboard v-if="activeTab === 'dashboard'" />
      <CampaignManager v-else-if="activeTab === 'campaign'" />
      <DecisionsPanel v-else-if="activeTab === 'decisions'" />
      <AgentPanel v-else-if="activeTab === 'agents'" />
      <NetworkMap v-else-if="activeTab === 'recon'" />
      <VulnerabilityHeatmap v-else-if="activeTab === 'vulns'" />
      <ServicePortTable v-else-if="activeTab === 'services'" />
      <PayloadBuilder v-else-if="activeTab === 'builder'" />
      <AutoModeMonitor v-else-if="activeTab === 'automode'" />
      <AIConsole v-else-if="activeTab === 'ai'" />
      <TerminalWidget v-else-if="activeTab === 'terminal'" />
      <MetricsPanel v-else-if="activeTab === 'metrics'" />
      <CredentialVault v-else-if="activeTab === 'creds'" />
      <CampaignTimeline v-else-if="activeTab === 'timeline'" />
      <DocsPanel v-else />
    </main>
    <Footer />
  </div>
</template>

<script setup>
import { ref, defineAsyncComponent, onMounted, onUnmounted } from 'vue'
import Header from './components/Header.vue'
import Tabs from './components/Tabs.vue'
import Footer from './components/Footer.vue'

const Dashboard = defineAsyncComponent(() => import('./views/Dashboard.vue'))
const CampaignManager = defineAsyncComponent(() => import('./views/CampaignManager.vue'))
const DecisionsPanel = defineAsyncComponent(() => import('./views/DecisionsPanel.vue'))
const AgentPanel = defineAsyncComponent(() => import('./views/AgentPanel.vue'))
const NetworkMap = defineAsyncComponent(() => import('./views/NetworkMap.vue'))
const VulnerabilityHeatmap = defineAsyncComponent(() => import('./views/VulnerabilityHeatmap.vue'))
const ServicePortTable = defineAsyncComponent(() => import('./views/ServicePortTable.vue'))
const AutoModeMonitor = defineAsyncComponent(() => import('./views/AutoModeMonitor.vue'))
const AIConsole = defineAsyncComponent(() => import('./views/AIConsole.vue'))
const PayloadBuilder = defineAsyncComponent(() => import('./views/PayloadBuilder.vue'))
const TerminalWidget = defineAsyncComponent(() => import('./views/TerminalWidget.vue'))
const MetricsPanel = defineAsyncComponent(() => import('./views/MetricsPanel.vue'))
const DocsPanel = defineAsyncComponent(() => import('./views/DocsPanel.vue'))
const CredentialVault = defineAsyncComponent(() => import('./views/CredentialVault.vue'))
const CampaignTimeline = defineAsyncComponent(() => import('./views/CampaignTimeline.vue'))
import {
  useAgentStore, useCampaignStore, useReconStore,
  useMetricsStore, useEventStore, useAIStore
} from './stores/index.js'

const activeTab = ref('dashboard')

let pollTimer = null

onMounted(() => {
  const agentStore = useAgentStore()
  const campaignStore = useCampaignStore()
  const reconStore = useReconStore()
  const metricsStore = useMetricsStore()
  const eventStore = useEventStore()
  const aiStore = useAIStore()

  // Initial fetch — all stores query the API
  agentStore.fetchAgents().catch(() => {})
  campaignStore.fetchCampaigns().catch(() => {})
  reconStore.fetchHosts().catch(() => {})
  reconStore.fetchServices().catch(() => {})
  reconStore.fetchVulnerabilities().catch(() => {})
  metricsStore.fetchMetrics().catch(() => {})
  metricsStore.fetchBlueMetrics().catch(() => {})

  // AI suggestions (async, non-blocking)
  setTimeout(() => {
    const cid = campaignStore.activeCampaign?.id
    if (cid) aiStore.getSuggestions(cid).catch(() => {})
  }, 2000)

  // Poll every 10 seconds for live updates
  pollTimer = setInterval(() => {
    agentStore.fetchAgents().catch(() => {})
    reconStore.fetchHosts().catch(() => {})
  }, 10000)

  // Connect WebSocket for real-time events
  eventStore.connect()
})

onUnmounted(() => {
  clearInterval(pollTimer)
  useEventStore().disconnect()
})
</script>
