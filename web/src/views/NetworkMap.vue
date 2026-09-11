<template>
  <div class="flex gap-3 h-full">
    <div class="glass-panel flex-1 p-4 relative overflow-hidden" ref="svgContainer">
      <div class="flex items-center justify-between mb-2">
        <h3 class="text-sm font-mono text-purple">NETWORK GRAPH (d3-force)</h3>
        <div class="flex gap-3 text-[10px] font-mono">
          <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full bg-neon"></span> Active</span>
          <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full bg-alert"></span> Compromised</span>
          <span class="flex items-center gap-1"><span class="w-2 h-2 rounded-full bg-purple"></span> Discovered</span>
        </div>
      </div>
      <svg ref="svgEl" width="100%" height="90%" class="cursor-move"></svg>
    </div>

    <div class="glass-panel w-80 p-3 flex flex-col text-xs font-mono overflow-y-auto space-y-1">
      <h3 class="text-purple mb-1">HOSTS ({{ hosts.length }})</h3>
      <div v-for="h in hosts" :key="h.ip"
        @click="selectedHost = selectedHost?.ip === h.ip ? null : h"
        class="p-2 rounded cursor-pointer border-l-2 transition-colors"
        :class="selectedHost?.ip === h.ip ? 'border-neon bg-dark/50' : 'border-transparent hover:bg-dark/30'">
        <div class="flex justify-between items-center">
          <span class="text-gray-300">{{ h.ip }}</span>
          <span :class="h.asset_value >= 70 ? 'text-alert' : h.asset_value >= 30 ? 'text-purple' : 'text-gray-500'">
            {{ h.asset_value }}
          </span>
        </div>
        <div class="text-gray-600 text-[10px]">{{ h.hostname || '?' }} · {{ h.os || '?' }}</div>
        <div v-if="hostVulns(h).length" class="mt-1 flex flex-wrap gap-1">
          <span v-for="v in hostVulns(h).slice(0, 3)" :key="v.cve || v.id"
            class="text-[9px] px-1 rounded" :class="sevClass(v.severity)">
            {{ v.cve_id || 'CVE' }}
          </span>
        </div>
        <div v-if="hostAgents(h).length" class="mt-1 text-[9px] text-alert">⚡ {{ hostAgents(h).length }} agent(s)</div>
      </div>

      <div v-if="selectedHost" class="mt-3 p-2 bg-dark rounded border border-gray-800 space-y-1">
        <h4 class="text-neon text-xs">{{ selectedHost.ip }}</h4>
        <div class="text-[10px] text-gray-500">
          <div>Hostname: {{ selectedHost.hostname || '?' }}</div>
          <div>OS: {{ selectedHost.os || '?' }}</div>
          <div>Risk: {{ selectedHost.asset_value }}/100</div>
          <div>Services: {{ selectedHost.port_count || (selectedHost.services || []).length || 0 }}</div>
          <div v-if="hostVulns(selectedHost).length">CVEs: {{ hostVulns(selectedHost).length }}</div>
          <div v-if="hostAgents(selectedHost).length">Agents: {{ hostAgents(selectedHost).join(', ') }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch, onUnmounted } from 'vue'
import { useReconStore, useAgentStore } from '../stores/index.js'
import * as d3 from 'd3'

const reconStore = useReconStore()
const agentStore = useAgentStore()

const svgContainer = ref(null)
const svgEl = ref(null)
const selectedHost = ref(null)
const hosts = computed(() => reconStore.hosts || [])

onMounted(() => {
  reconStore.fetchHosts().catch(() => {})
  reconStore.fetchVulnerabilities().catch(() => {})
})
watch([() => hosts.value.length, () => reconStore.vulnerabilities.length], () => {
  if (hosts.value.length) drawGraph()
})
onMounted(() => {
  if (hosts.value.length) drawGraph()
})

function hostVulns(h) {
  return (reconStore.vulnerabilities || []).filter(v =>
    v.target_ip === h.ip
  )
}
function hostAgents(h) {
  return (agentStore.agents || []).filter(a =>
    a.local_ip === h.ip && (a.status === 'online' || a.status === 'active')
  )
}
function sevClass(s) {
  const m = { 'CRITICAL': 'bg-red-900/30 text-red-300', 'HIGH': 'bg-orange-900/20 text-orange-400', 'MEDIUM': 'bg-yellow-900/20 text-yellow-400', 'LOW': 'bg-neon/10 text-neon' }
  return m[s?.toUpperCase()] || 'bg-gray-800 text-gray-400'
}

let simulation = null

function drawGraph() {
  if (!svgEl.value) return
  const svg = d3.select(svgEl.value)
  svg.selectAll('*').remove()
  if (simulation) { simulation.stop(); simulation = null }

  const rect = svgEl.value.parentElement.getBoundingClientRect()
  const w = Math.max(rect.width || 500, 300)
  const h = Math.max(rect.height || 400, 300)

  const nodesData = hosts.value.map((h, i) => ({
    id: h.ip,
    label: h.hostname || h.ip?.slice(-6) || `H${i}`,
    group: h.os || 'unknown',
    compromised: hostAgents(h).length > 0,
    vulnCount: hostVulns(h).length,
    vulnMax: hostVulns(h).some(v => v.severity === 'CRITICAL' || v.severity === 'HIGH'),
    assetValue: h.asset_value || 0,
  }))

  // real topology only: no invented chain links — hosts stand alone
  // until lateral-movement edges exist in the world graph
  const linksData = []

  simulation = d3.forceSimulation(nodesData)
    .force('link', d3.forceLink(linksData).id(d => d.id).distance(100))
    .force('charge', d3.forceManyBody().strength(-200))
    .force('center', d3.forceCenter(w / 2, h / 2))
    .force('collision', d3.forceCollide().radius(35))

  const zoom = d3.zoom().scaleExtent([0.2, 4]).on('zoom', (e) => g.attr('transform', e.transform))
  svg.call(zoom)

  const defs = svg.append('defs')
  defs.append('radialGradient').attr('id', 'node-gradient').attr('cx', '30%').attr('cy', '30%')
    .append('stop').attr('offset', '0%').attr('stop-color', 'rgba(255,255,255,0.15)')
  d3.select('#node-gradient').append('stop').attr('offset', '100%').attr('stop-color', 'transparent')

  const g = svg.append('g')

  const link = g.append('g').selectAll('line').data(linksData).join('line')
    .attr('stroke', 'rgba(108,99,255,0.15)').attr('stroke-width', 1.5).attr('stroke-dasharray', '4,3')

  const node = g.append('g').selectAll('g').data(nodesData).join('g')
    .call(d3.drag()
      .on('start', (e, d) => { if (!e.active) simulation.alphaTarget(0.3).restart(); d.fx = d.x; d.fy = d.y })
      .on('drag', (e, d) => { d.fx = e.x; d.fy = e.y })
      .on('end', (e, d) => { if (!e.active) simulation.alphaTarget(0); d.fx = null; d.fy = null })
    )

  node.append('circle')
    .attr('r', d => Math.max(18, Math.min(30, 16 + d.vulnCount * 3)))
    .attr('fill', d => d.compromised ? '#ff4444' : d.vulnMax ? '#ff8800' : d.assetValue > 50 ? '#6c63ff' : '#334')
    .attr('stroke', d => d.compromised ? '#ff6666' : d.vulnMax ? '#ffaa44' : 'rgba(255,255,255,0.15)')
    .attr('stroke-width', 2)

  node.append('text')
    .text(d => d.label)
    .attr('text-anchor', 'middle')
    .attr('y', 4)
    .attr('fill', 'white')
    .attr('font-size', '9px')
    .attr('font-family', 'monospace')
    .attr('font-weight', 'bold')

  node.append('text')
    .text(d => d.vulnCount ? `${d.vulnCount}CVE` : '')
    .attr('text-anchor', 'middle')
    .attr('y', -22)
    .attr('fill', '#ff8844')
    .attr('font-size', '7px')
    .attr('font-family', 'monospace')

  simulation.on('tick', () => {
    link
      .attr('x1', d => d.source.x).attr('y1', d => d.source.y)
      .attr('x2', d => d.target.x).attr('y2', d => d.target.y)
    node.attr('transform', d => `translate(${d.x},${d.y})`)
  })
}

onUnmounted(() => { if (simulation) simulation.stop() })
</script>
