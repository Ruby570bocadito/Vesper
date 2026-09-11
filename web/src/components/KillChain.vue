<template>
  <div class="glass-panel p-4">
    <h3 class="text-sm font-mono text-purple mb-3">KILL CHAIN PHASES</h3>
    <div class="flex items-center gap-2 text-xs font-mono flex-wrap">
      <div v-for="(phase, i) in phases" :key="i" class="flex items-center gap-2 transition-all duration-300">
        <span :class="phaseClass(phase.status)">{{ phase.icon }}</span>
        <span :class="phaseTextClass(phase.status)">{{ phase.name }}</span>
        <span v-if="i < phases.length - 1" class="text-gray-700 mx-1">─</span>
      </div>
    </div>
    <div class="mt-3 bg-dark rounded-full h-2 overflow-hidden">
      <div class="h-full bg-gradient-to-r from-purple to-neon rounded-full transition-all duration-700 ease-out"
        :style="{ width: progressPct + '%' }"></div>
    </div>
    <div class="flex justify-between text-xs font-mono mt-1">
      <span class="text-gray-600">{{ progressPct }}%</span>
      <span class="text-purple">{{ currentLabel }}</span>
      <span class="text-gray-600" v-if="lastEvent">⚡ {{ lastEvent }}</span>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useCampaignStore, useEventStore } from '../stores/index.js'

const campaignStore = useCampaignStore()
const eventStore = useEventStore()
const lastEvent = ref('')
let wsUnsub = null

onMounted(() => {
  campaignStore.fetchCampaigns().catch(() => {})

  // Subscribe to live events for phase transitions
  if (!eventStore.connected) eventStore.connect()
  wsUnsub = setInterval(() => {
    const phaseEvents = eventStore.events.filter(e =>
      e.type === 'phase.changed' || e.type === 'campaign.started'
    )
    if (phaseEvents.length) {
      lastEvent.value = phaseEvents[0].description || phaseEvents[0].phase || ''
    }
  }, 2000)
})

onUnmounted(() => { if (wsUnsub) clearInterval(wsUnsub) })

const phaseOrder = { recon: 0, weaponization: 1, delivery: 2, exploitation: 3, installation: 4, c2: 5, actions_on_objective: 6, exfiltration: 7 }
const phaseLabels = ['Recon', 'Weaponize', 'Deliver', 'Exploit', 'Install', 'C2', 'Actions', 'Exfil']

const currentPhase = computed(() => campaignStore.activeCampaign?.phase || 'recon')
const currentIdx = computed(() => phaseOrder[currentPhase.value] ?? 0)
const progressPct = computed(() => Math.round(((currentIdx.value + 1) / 8) * 100))
const currentLabel = computed(() => phaseLabels[currentIdx.value] || currentPhase.value)

const phases = computed(() => {
  const idx = currentIdx.value
  return phaseLabels.slice(0, 7).map((name, i) => ({
    name,
    icon: i < idx ? '✓' : i === idx ? '◉' : '□',
    status: i < idx ? 'done' : i === idx ? 'active' : 'pending'
  }))
})

const phaseClass = (s) => ({ done: 'text-neon', active: 'text-yellow-400 animate-pulse-slow', pending: 'text-gray-700' }[s])
const phaseTextClass = (s) => ({ done: 'text-neon', active: 'text-yellow-400', pending: 'text-gray-600' }[s])
</script>
