<template>
  <footer class="glass-panel flex items-center justify-between px-4 py-1 m-3 mt-0 text-xs text-gray-600 font-mono">
    <span>⬡ Vesper — Rafael Gálvez | Cisco NetAcad | TFG Cybersecurity</span>
    <span>
      {{ now }} |
      Agents: {{ agentStore.agentCount }} |
      Sessions: {{ agentStore.activeAgents.length }} |
      Hosts: {{ reconStore.hosts.length }} |
      {{ eventStore.connected ? 'WS connected' : 'API disconnected' }}
    </span>
  </footer>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useAgentStore, useReconStore, useEventStore } from '../stores/index.js'

const agentStore = useAgentStore()
const reconStore = useReconStore()
const eventStore = useEventStore()

const now = ref(new Date().toLocaleTimeString())
let interval = null

onMounted(() => {
  interval = setInterval(() => { now.value = new Date().toLocaleTimeString() }, 1000)
})
onUnmounted(() => clearInterval(interval))
</script>
