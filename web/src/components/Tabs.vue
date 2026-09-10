<template>
  <nav class="flex gap-1 px-3 pb-2">
    <button v-for="tab in tabs" :key="tab.id" @click="$emit('switch', tab.id)"
      class="px-4 py-2 text-sm font-mono rounded-t-lg transition-all"
      :class="tab.id === active ? 'text-neon border-b-2 border-neon bg-panel' : 'text-gray-500 hover:text-gray-300'">
      {{ tab.label }}
      <span v-if="tab.badge !== null" class="ml-1 text-xs text-neon">● {{ tab.badge }}</span>
    </button>
  </nav>
</template>

<script setup>
import { computed } from 'vue'
import { useAgentStore, useReconStore, useEventStore, useAIStore } from '../stores/index.js'

const props = defineProps({ active: { type: String, default: 'dashboard' } })
defineEmits(['switch'])

const agentStore = useAgentStore()
const reconStore = useReconStore()
const eventStore = useEventStore()
const aiStore = useAIStore()

const tabs = computed(() => [
  { id: 'dashboard', label: 'Dashboard', badge: null },
  { id: 'campaign', label: 'Campaigns', badge: null },
  { id: 'decisions', label: 'Decisions', badge: (aiStore.suggestions || []).filter(d => !d.approved && !d.rejected).length },
  { id: 'agents', label: 'Agents', badge: agentStore.activeAgents.length },
  { id: 'recon', label: 'Graph', badge: reconStore.hosts.length },
  { id: 'vulns', label: 'Vulns', badge: reconStore.vulnerabilities.length },
  { id: 'services', label: 'Services', badge: reconStore.services.length },
  { id: 'automode', label: 'AutoMode', badge: null },
  { id: 'timeline', label: 'Timeline', badge: eventStore.events.length },
  { id: 'builder', label: 'Payloads', badge: null },
  { id: 'ai', label: 'AI', badge: null },
  { id: 'creds', label: 'Creds', badge: null },
  { id: 'terminal', label: 'Terminal', badge: null },
  { id: 'metrics', label: 'Metrics', badge: null },
  { id: 'docs', label: 'Docs', badge: null },
])
</script>
