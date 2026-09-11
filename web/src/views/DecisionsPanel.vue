<template>
  <div class="h-full flex flex-col">
    <div class="flex items-center justify-between mb-3">
      <h2 class="text-lg font-mono text-purple">AI DECISION COMMAND</h2>
      <div class="flex gap-2 items-center">
        <button @click="refreshSuggestions" :disabled="loading" class="btn text-xs px-3 py-1">⟳ Refresh</button>
        <span class="text-xs font-mono text-gray-600">{{ decisions.length }} pending</span>
      </div>
    </div>

    <div class="flex-1 overflow-y-auto space-y-2">
      <div v-if="loading" class="text-gray-600 text-center py-8 font-mono text-sm">
        Requesting AI evaluation...
      </div>
      <div v-else-if="decisions.length === 0" class="text-gray-600 text-center py-8 font-mono text-sm">
        <div class="text-4xl mb-3">🤖</div>
        No pending decisions. Deploy agents to trigger AI evaluation.
      </div>

      <div v-for="d in decisions" :key="d.id" class="glass-panel p-3" :class="decisionBorder(d)">
        <div class="flex items-start gap-3">
          <div class="flex-1">
            <div class="flex items-center gap-2 mb-1">
              <span class="text-xs font-mono px-1.5 py-0.5 rounded" :class="tacticBadge(d.tactic)">
                {{ d.tactic || 'Unknown' }}
              </span>
              <span class="text-xs font-mono text-gray-400">{{ d.technique || '?' }}</span>
              <span v-if="d.mitre_id" class="text-[10px] font-mono text-purple">{{ d.mitre_id }}</span>
              <span class="text-[10px] px-1 py-0.5 rounded font-mono" :class="riskBadge(d.risk)">
                {{ d.risk || 'LOW' }}
              </span>
            </div>
            <p class="text-xs text-gray-300 font-mono mb-1">{{ d.reasoning || 'No reasoning provided' }}</p>
            <div class="flex items-center gap-3 text-[10px] font-mono text-gray-600">
              <span>Confidence: {{ Math.round((d.confidence || 0) * 100) }}%</span>
              <span>Source: {{ d.source || 'rules' }}</span>
              <span v-if="d.target">Target: {{ d.target }}</span>
            </div>
          </div>

          <div class="flex flex-col gap-1" v-if="!d.approved && !d.rejected">
            <button @click="approve(d.id)" :disabled="acting"
              class="text-xs px-3 py-1 rounded font-mono bg-neon/10 text-neon border border-neon/30 hover:bg-neon/20 transition-colors">
              APPROVE
            </button>
            <button @click="reject(d.id)" :disabled="acting"
              class="text-xs px-3 py-1 rounded font-mono bg-red-900/20 text-red-400 border border-red-800/30 hover:bg-red-900/30 transition-colors">
              REJECT
            </button>
          </div>
          <div v-else class="flex flex-col items-center gap-1">
            <span v-if="d.approved" class="text-neon text-lg">✓</span>
            <span v-if="d.rejected" class="text-red-400 text-lg">✗</span>
            <span class="text-[10px] font-mono text-gray-600">{{ d.approved ? 'Approved' : 'Rejected' }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useAIStore } from '../stores/index.js'

const aiStore = useAIStore()
const decisions = ref([])
const loading = ref(false)
const acting = ref(false)

onMounted(() => refreshSuggestions())

async function refreshSuggestions() {
  loading.value = true
  try {
    await aiStore.getSuggestions()
    decisions.value = (aiStore.suggestions || []).map((d, i) => ({
      ...d,
      id: d.id || d.decision_id || `dec-${i}`,
    }))
  } finally {
    loading.value = false
  }
}

async function approve(id) {
  acting.value = true
  try {
    const ok = await aiStore.approveDecision(id)
    if (ok) {
      const d = decisions.value.find(x => x.id === id)
      if (d) d.approved = true
    }
  } catch { /* UI must not claim success on a failed request */ }
  finally { acting.value = false }
}

async function reject(id) {
  acting.value = true
  try {
    const ok = await aiStore.rejectDecision(id)
    if (ok) {
      const d = decisions.value.find(x => x.id === id)
      if (d) d.rejected = true
    }
  } catch { /* UI must not claim success on a failed request */ }
  finally { acting.value = false }
}

function decisionBorder(d) {
  if (d.approved) return 'border-l-2 border-neon'
  if (d.rejected) return 'border-l-2 border-red-500'
  if ((d.confidence || 0) > 0.7) return 'border-l-2 border-yellow-400'
  return 'border-l-2 border-transparent'
}

function tacticBadge(t) {
  const m = {
    'Initial Access': 'bg-red-900/20 text-red-400 border border-red-800/30',
    'Execution': 'bg-orange-900/20 text-orange-400 border border-orange-800/30',
    'Persistence': 'bg-green-900/20 text-green-400 border border-green-800/30',
    'Privilege Escalation': 'bg-yellow-900/20 text-yellow-400 border border-yellow-800/30',
    'Defense Evasion': 'bg-purple/20 text-purple border border-purple/30',
    'Credential Access': 'bg-blue-900/20 text-blue-400 border border-blue-800/30',
    'Discovery': 'bg-cyan-900/20 text-cyan-400 border border-cyan-800/30',
    'Lateral Movement': 'bg-pink-900/20 text-pink-400 border border-pink-800/30',
    'Collection': 'bg-teal-900/20 text-teal-400 border border-teal-800/30',
    'Command and Control': 'bg-indigo-900/20 text-indigo-400 border border-indigo-800/30',
    'Exfiltration': 'bg-red-900/20 text-red-400 border border-red-800/30',
  }
  return m[t] || 'bg-gray-800 text-gray-400 border border-gray-700'
}

function riskBadge(r) {
  const m = {
    'SAFE': 'bg-neon/10 text-neon border border-neon/30',
    'LOW': 'bg-neon/10 text-neon border border-neon/30',
    'MEDIUM': 'bg-yellow-900/20 text-yellow-400 border border-yellow-800/30',
    'HIGH': 'bg-red-900/20 text-red-400 border border-red-800/30',
    'DANGER': 'bg-red-900/30 text-red-300 border border-red-700/30',
  }
  return m[r?.toUpperCase?.()] || m['LOW']
}
</script>
