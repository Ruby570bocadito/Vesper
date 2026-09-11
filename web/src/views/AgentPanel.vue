<template>
  <div class="flex gap-3 h-full">
    <div class="glass-panel p-4 flex-1 overflow-hidden flex flex-col">
      <div class="flex items-center justify-between mb-3">
        <h3 class="text-sm font-mono text-purple">AGENT FLEET</h3>
        <span class="text-xs text-gray-600 font-mono">{{ store.agentCount }} agents</span>
      </div>
      <div class="flex-1 overflow-auto">
        <div v-if="store.loading" class="text-center text-gray-500 text-xs font-mono py-8">
          Loading agents...
        </div>
        <div v-else-if="store.error" class="text-center text-alert text-xs font-mono py-8">
          {{ store.error }}
        </div>
        <table v-else class="w-full text-xs font-mono">
          <thead>
            <tr class="text-gray-600 border-b border-gray-800">
              <th class="text-left py-2">ID</th>
              <th class="text-left">Hostname</th>
              <th class="text-left">OS</th>
              <th class="text-left">User</th>
              <th class="text-left">IP</th>
              <th class="text-left">Status</th>
              <th class="text-left">Uptime</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="agent in store.agents" :key="agent.id" @click="selected = selected === agent.id ? null : agent.id"
              class="border-b border-gray-800/50 hover:bg-panel cursor-pointer"
              :class="selected === agent.id ? 'bg-dark/50' : ''">
              <td class="py-2 text-neon">{{ agent.id }}</td>
              <td class="text-gray-300">{{ agent.hostname }}</td>
              <td class="text-gray-400">{{ agent.os }}</td>
              <td :class="agent.username === 'root' || (agent.username && agent.username.includes('SYSTEM')) ? 'text-alert' : 'text-gray-300'">{{ agent.username }}</td>
              <td class="text-gray-400">{{ agent.local_ip }}</td>
              <td>
                <span class="inline-block w-2 h-2 rounded-full mr-1" :class="statusColor(agent.status)"></span>
                <span :class="statusTextColor(agent.status)">{{ agent.status }}</span>
              </td>
              <td class="text-gray-500">{{ agent.uptime }}h</td>
              <td>
                <button v-if="agent.status !== 'dead'" @click.stop="kill(agent)" class="btn btn-reject text-xs px-2 py-0.5">Kill</button>
              </td>
            </tr>
            <tr v-if="store.agents.length === 0">
              <td colspan="8" class="text-center text-gray-600 py-8">No agents connected. Deploy an agent to get started.</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Lateral Detail Panel -->
    <div v-if="selectedAgent" class="glass-panel w-80 p-3 overflow-y-auto text-xs font-mono space-y-3 shrink-0 border-l border-purple/30">
      <div class="flex items-center justify-between">
        <h4 class="text-neon text-sm">{{ selectedAgent.id }}</h4>
        <button @click="selected = null" class="text-gray-600 hover:text-white">✕</button>
      </div>

      <div class="space-y-2">
        <div class="p-2 bg-dark rounded">
          <div class="text-gray-600 text-[10px]">Status</div>
          <div class="flex items-center gap-1">
            <span class="inline-block w-2 h-2 rounded-full" :class="statusColor(selectedAgent.status)"></span>
            <span :class="statusTextColor(selectedAgent.status)">{{ selectedAgent.status }}</span>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-2">
          <div class="p-2 bg-dark rounded">
            <div class="text-gray-600 text-[10px]">OS</div>
            <div class="text-gray-300">{{ selectedAgent.os || '?' }}</div>
          </div>
          <div class="p-2 bg-dark rounded">
            <div class="text-gray-600 text-[10px]">Arch</div>
            <div class="text-gray-300">{{ selectedAgent.arch || selectedAgent.architecture || 'x64' }}</div>
          </div>
          <div class="p-2 bg-dark rounded">
            <div class="text-gray-600 text-[10px]">Privileges</div>
            <div :class="selectedAgent.user === 'root' || (selectedAgent.user && selectedAgent.user.includes('SYSTEM')) ? 'text-alert' : 'text-gray-300'">
              {{ (selectedAgent.privileges && selectedAgent.privileges.length) ? selectedAgent.privileges.join(', ') : '?' }}
            </div>
          </div>
          <div class="p-2 bg-dark rounded">
            <div class="text-gray-600 text-[10px]">Uptime</div>
            <div class="text-gray-300">{{ selectedAgent.uptime }}h</div>
          </div>
        </div>
        <div class="p-2 bg-dark rounded">
          <div class="text-gray-600 text-[10px]">Internal IP</div>
          <div class="text-gray-300">{{ selectedAgent.local_ip }}</div>
        </div>
        <div class="p-2 bg-dark rounded">
          <div class="text-gray-600 text-[10px]">External IP</div>
          <div class="text-gray-300">{{ selectedAgent.hostname || '?' }}</div>
        </div>
        <div class="p-2 bg-dark rounded">
          <div class="text-gray-600 text-[10px]">Hostname</div>
          <div class="text-gray-300">{{ selectedAgent.hostname || '?' }}</div>
        </div>
        <div class="p-2 bg-dark rounded">
          <div class="text-gray-600 text-[10px]">Domain</div>
          <div class="text-gray-300">{{ selectedAgent.domain || 'WORKGROUP' }}</div>
        </div>
        <div v-if="false" class="p-2 bg-dark rounded">
          <div class="text-gray-600 text-[10px]">Integrity</div>
          <div class="text-gray-300">{{ selectedAgent.integrity_level }}</div>
        </div>
        <div v-if="selectedAgent.last_checkin" class="p-2 bg-dark rounded">
          <div class="text-gray-600 text-[10px]">Last Seen</div>
          <div class="text-gray-300">{{ new Date(selectedAgent.last_checkin).toLocaleString() }}</div>
        </div>
      </div>

      <div class="flex gap-2 pt-1">
        <button @click="interact(selectedAgent)" class="btn flex-1 text-[10px] py-1" title="Interactive shells land in v1.2" disabled>Interactive (v1.2)</button>
        <button @click="kill(selectedAgent)" class="btn btn-reject flex-1 text-[10px] py-1">Kill</button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useAgentStore } from '../stores/index.js'

const store = useAgentStore()
const selected = ref(null)

const selectedAgent = computed(() => {
  if (!selected.value) return null
  return store.agents.find(a => a.id === selected.value) || null
})

onMounted(() => store.fetchAgents().catch(() => {}))

const kill = (agent) => {
  if (confirm(`Kill agent ${agent.id}?`)) {
    store.killAgent(agent.id, 'user requested')
    if (selected.value === agent.id) selected.value = null
  }
}

const interact = (agent) => {
  // honest: no interactive shell endpoint exists yet; button stays disabled
  console.warn('interactive shells not implemented (roadmap v1.2)', agent?.id)
}

const statusColor = (s) => ({ online: 'bg-neon', active: 'bg-yellow-400', idle: 'bg-gray-500', dead: 'bg-alert' }[s] || 'bg-gray-500')
const statusTextColor = (s) => ({ online: 'text-neon', active: 'text-yellow-400', idle: 'text-gray-500', dead: 'text-alert' }[s] || 'text-gray-500')
</script>
