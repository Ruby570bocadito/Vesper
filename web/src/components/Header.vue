<template>
  <header class="glass-panel flex items-center justify-between px-6 py-2 m-3 mb-0">
    <div class="flex items-center gap-3">
      <span class="text-2xl animate-glitch">⬡</span>
      <h1 class="text-xl font-mono font-bold neon-purple">Vesper</h1>
      <span class="text-sm text-gray-500 font-mono">v1.0</span>
    </div>
    <div class="flex items-center gap-4">
      <div class="text-sm">
        <span class="text-gray-500">CAMPAIGN:</span>
        <span class="text-purple ml-1">{{ campaignStore.activeCampaign?.name || 'No active' }}</span>
        <span class="text-gray-600 mx-2">·</span>
        <span class="text-gray-500">PHASE:</span>
        <span class="text-neon ml-1">{{ campaignStore.activeCampaign?.phase || '—' }}</span>
      </div>
      <div class="flex items-center gap-2">
        <span class="inline-block w-2 h-2 rounded-full bg-neon animate-pulse-slow"></span>
        <span class="text-sm text-neon font-mono">{{ agentStore.activeAgents.length }} agents</span>
      </div>
      <div class="flex items-center gap-2">
        <span :class="eventStore.connected ? 'bg-neon' : 'bg-gray-600'" class="inline-block w-2 h-2 rounded-full animate-pulse-slow"></span>
        <span :class="eventStore.connected ? 'text-purple' : 'text-gray-600'" class="text-sm font-mono">
          {{ eventStore.connected ? 'C2 online' : 'offline' }}
        </span>
      </div>
    </div>
  </header>
</template>

<script setup>
import { useAgentStore, useCampaignStore, useEventStore } from '../stores/index.js'

const agentStore = useAgentStore()
const campaignStore = useCampaignStore()
const eventStore = useEventStore()
</script>
