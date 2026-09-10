package orchestrator

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/ruby570bocadito/vesper/pkg/shared/types"
)

// WorldGraph maintains a live graph of the target environment.
// Nodes are hosts, edges represent network connectivity and exploits.
type WorldGraph struct {
	mu       sync.RWMutex
	nodes    map[string]*WorldNode
	edges    map[string][]*WorldEdge
	services map[string]map[int]*WorldService // IP -> port -> service
}

// WorldNode represents a host in the target network.
type WorldNode struct {
	IP          string         `json:"ip"`
	Hostname    string         `json:"hostname"`
	OS          string         `json:"os"`
	Status      string         `json:"status"`
	ServiceList []string       `json:"services"`
	Tags        map[string]int `json:"tags"`
	Compromised bool           `json:"compromised"`
}

// WorldEdge represents a relationship between two hosts.
type WorldEdge struct {
	From    string  `json:"from"`
	To      string  `json:"to"`
	Type    string  `json:"type"`
	Exploit string  `json:"exploit,omitempty"`
	Success float64 `json:"success"`
}

// WorldService represents a service running on a host.
type WorldService struct {
	Name    string `json:"name"`
	Port    int    `json:"port"`
	Version string `json:"version"`
	Banner  string `json:"banner,omitempty"`
}

func NewWorldGraph() *WorldGraph {
	return &WorldGraph{
		nodes:    make(map[string]*WorldNode),
		edges:    make(map[string][]*WorldEdge),
		services: make(map[string]map[int]*WorldService),
	}
}

func (wg *WorldGraph) AddHost(ip, hostname, os string) {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	if node, exists := wg.nodes[ip]; exists {
		if hostname != "" {
			node.Hostname = hostname
		}
		if os != "" && os != "unknown" {
			node.OS = os
		}
		return
	}

	wg.nodes[ip] = &WorldNode{
		IP:       ip,
		Hostname: hostname,
		OS:       os,
		Status:   "discovered",
		Tags:     make(map[string]int),
	}

	if _, exists := wg.services[ip]; !exists {
		wg.services[ip] = make(map[int]*WorldService)
	}
}

func (wg *WorldGraph) AddService(ip string, svc WorldService) {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	if _, exists := wg.services[ip]; !exists {
		wg.services[ip] = make(map[int]*WorldService)
	}
	wg.services[ip][svc.Port] = &svc
}

func (wg *WorldGraph) MarkCompromised(ip string) {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	if node, ok := wg.nodes[ip]; ok {
		node.Compromised = true
		node.Status = "compromised"
	}
}

func (wg *WorldGraph) AddEdge(from, to, edgeType string, success float64) {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	edge := &WorldEdge{
		From:    from,
		To:      to,
		Type:    edgeType,
		Success: success,
	}

	wg.edges[from] = append(wg.edges[from], edge)

	// Only add reverse edge for bidirectional connectivity (e.g., "network")
	// Exploits and other unidirectional edges shouldn't have a reverse path added automatically.
	if edgeType == "network" || edgeType == "route" {
		reverse := &WorldEdge{
			From:    to,
			To:      from,
			Type:    edgeType,
			Success: success,
		}
		wg.edges[to] = append(wg.edges[to], reverse)
	}
}

func (wg *WorldGraph) AddExploitEdge(from, to, exploit string, success float64) {
	wg.mu.Lock()
	defer wg.mu.Unlock()

	edge := &WorldEdge{
		From:    from,
		To:      to,
		Type:    "exploit",
		Exploit: exploit,
		Success: success,
	}
	wg.edges[from] = append(wg.edges[from], edge)
}

func (wg *WorldGraph) GetNode(ip string) (*WorldNode, error) {
	wg.mu.RLock()
	defer wg.mu.RUnlock()

	node, ok := wg.nodes[ip]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", ip)
	}
	return node, nil
}

func (wg *WorldGraph) GetAllNodes() []*WorldNode {
	wg.mu.RLock()
	defer wg.mu.RUnlock()

	nodes := make([]*WorldNode, 0, len(wg.nodes))
	for _, node := range wg.nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].IP < nodes[j].IP
	})
	return nodes
}

func (wg *WorldGraph) GetServices(ip string) []WorldService {
	wg.mu.RLock()
	defer wg.mu.RUnlock()

	svcMap, ok := wg.services[ip]
	if !ok {
		return nil
	}

	services := make([]WorldService, 0, len(svcMap))
	for _, svc := range svcMap {
		services = append(services, *svc)
	}
	sort.Slice(services, func(i, j int) bool {
		return services[i].Port < services[j].Port
	})
	return services
}

func (wg *WorldGraph) GetAllServices() []WorldService {
	wg.mu.RLock()
	defer wg.mu.RUnlock()

	var allServices []WorldService
	for _, svcMap := range wg.services {
		for _, svc := range svcMap {
			allServices = append(allServices, *svc)
		}
	}
	return allServices
}

func (wg *WorldGraph) GetEdges(from, to string) []*WorldEdge {
	wg.mu.RLock()
	defer wg.mu.RUnlock()

	var result []*WorldEdge
	for _, edge := range wg.edges[from] {
		if edge.To == to {
			result = append(result, edge)
		}
	}
	return result
}

func (wg *WorldGraph) GetNeighbors(ip string) []string {
	wg.mu.RLock()
	defer wg.mu.RUnlock()

	var neighbors []string
	seen := make(map[string]bool)

	for _, edge := range wg.edges[ip] {
		if !seen[edge.To] {
			neighbors = append(neighbors, edge.To)
			seen[edge.To] = true
		}
	}

	return neighbors
}

func (wg *WorldGraph) NodeCount() int {
	wg.mu.RLock()
	defer wg.mu.RUnlock()
	return len(wg.nodes)
}

func (wg *WorldGraph) EdgeCount() int {
	wg.mu.RLock()
	defer wg.mu.RUnlock()

	count := 0
	for _, edges := range wg.edges {
		count += len(edges)
	}
	return count
}

// Summary returns a human-readable summary of the world graph state.
func (wg *WorldGraph) Summary() string {
	wg.mu.RLock()
	defer wg.mu.RUnlock()

	var sb strings.Builder

	fmt.Fprintf(&sb, "Nodes: %d | Edges: %d\n", len(wg.nodes), wg.edgeCountUnsafe())

	for _, node := range wg.nodes {
		status := "○"
		if node.Compromised {
			status = "●"
		}
		fmt.Fprintf(&sb, "  %s %s (%s)", status, node.IP, node.OS)
		if node.Hostname != "" {
			fmt.Fprintf(&sb, " [%s]", node.Hostname)
		}

		services := wg.services[node.IP]
		if len(services) > 0 {
			fmt.Fprintf(&sb, " — %d services", len(services))
			for _, svc := range services {
				fmt.Fprintf(&sb, " %s:%d", svc.Name, svc.Port)
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (wg *WorldGraph) edgeCountUnsafe() int {
	count := 0
	for _, edges := range wg.edges {
		count += len(edges)
	}
	return count
}

// GenerateDemoData populates the world graph with sample data for testing.
func (wg *WorldGraph) GenerateDemoData() {
	// Add hosts
	wg.AddHost("10.0.0.10", "DC", "Windows Server 2019")
	wg.AddHost("10.0.0.20", "DB", "Ubuntu 24.04")
	wg.AddHost("10.0.0.50", "WS1", "Windows 11")
	wg.AddHost("10.0.0.51", "WS2", "Windows 11")
	wg.AddHost("10.0.0.30", "WEB", "CentOS 8")

	// Add services
	wg.AddService("10.0.0.10", WorldService{Name: "smb", Port: 445, Version: "SMBv1"})
	wg.AddService("10.0.0.10", WorldService{Name: "rdp", Port: 3389, Version: "RDP 10.0"})
	wg.AddService("10.0.0.10", WorldService{Name: "dns", Port: 53, Version: "Windows DNS"})
	wg.AddService("10.0.0.20", WorldService{Name: "ssh", Port: 22, Version: "OpenSSH 9.6"})
	wg.AddService("10.0.0.20", WorldService{Name: "mysql", Port: 3306, Version: "MySQL 8.0"})
	wg.AddService("10.0.0.20", WorldService{Name: "redis", Port: 6379, Version: "Redis 7.2"})
	wg.AddService("10.0.0.30", WorldService{Name: "http", Port: 80, Version: "Apache 2.4.49"})
	wg.AddService("10.0.0.50", WorldService{Name: "smb", Port: 445, Version: "SMBv2"})

	// Add edges (network connectivity)
	wg.AddEdge("10.0.0.10", "10.0.0.50", "network", 1.0)
	wg.AddEdge("10.0.0.10", "10.0.0.51", "network", 1.0)
	wg.AddEdge("10.0.0.10", "10.0.0.20", "network", 1.0)
	wg.AddEdge("10.0.0.50", "10.0.0.30", "network", 1.0)

	// Add exploit edges once services are discovered
	wg.AddExploitEdge("10.0.0.10", "10.0.0.50", "EternalBlue (MS17-010)", 0.85)
	wg.AddExploitEdge("10.0.0.10", "10.0.0.20", "SSH Credential Reuse", 0.75)
	wg.AddExploitEdge("10.0.0.20", "10.0.0.30", "Web App CVE-2021-41773", 0.90)
}

// DiscoverFromAgents populates the world graph from live agent data.
func (wg *WorldGraph) DiscoverFromAgents(agents []*types.Agent) {
	for _, a := range agents {
		hostname := a.Hostname
		if hostname == "" {
			hostname = a.ID
		}
		osName := a.OS
		if osName == "" {
			osName = "unknown"
		}
		ip := a.LocalIP
		if ip == "" {
			ip = a.ID
		}
		wg.AddHost(ip, hostname, osName)
	}
}
