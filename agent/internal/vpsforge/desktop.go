package vpsforge

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"vit-daw-agent/internal/vps"
)

// DesktopOptions supplies only local, staging-scoped dependencies to the
// browser workbench. It deliberately has no Credential, Catalog, Library, or
// SPAL-routing dependency.
type DesktopOptions struct {
	StagingRoot       string
	WitnessExecutable string
	LaunchWitness     func(pluginPath string) (int, error)
}

type DesktopManager struct {
	mu                sync.RWMutex
	root              string
	stagingRoot       string
	witnessExecutable string
	launchWitness     func(pluginPath string) (int, error)
}

func NewDesktopManager(options ...DesktopOptions) *DesktopManager {
	manager := &DesktopManager{}
	if len(options) > 0 {
		manager.stagingRoot = strings.TrimSpace(options[0].StagingRoot)
		manager.witnessExecutable = strings.TrimSpace(options[0].WitnessExecutable)
		manager.launchWitness = options[0].LaunchWitness
	}
	return manager
}

type desktopWorkspace struct {
	Workspace       string    `json:"workspace"`
	Manufacturer    string    `json:"manufacturer"`
	PluginName      string    `json:"plugin_name"`
	Format          string    `json:"format"`
	Version         string    `json:"version"`
	CandidateBadges []string  `json:"candidate_badges"`
	CreatedAt       time.Time `json:"created_at"`
	Launchable      bool      `json:"launchable"`
}

func (m *DesktopManager) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(desktopHTML))
	})
	mux.HandleFunc("/api/init", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeHTTP(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
			return
		}
		var request struct {
			Workspace    string   `json:"workspace"`
			Manufacturer string   `json:"manufacturer"`
			Name         string   `json:"name"`
			Format       string   `json:"format"`
			Version      string   `json:"version"`
			InstallPath  string   `json:"install_path"`
			Capabilities []string `json:"capabilities"`
		}
		if err := decodeHTTP(r, &request); err != nil {
			writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		status, err := Init(InitRequest{Root: request.Workspace, Identity: vps.PluginIdentity{Manufacturer: request.Manufacturer, Name: request.Name, Format: request.Format, Version: request.Version, InstallPath: request.InstallPath}, Capabilities: request.Capabilities})
		if err != nil {
			writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		m.setRoot(request.Workspace)
		writeHTTP(w, http.StatusOK, status)
	})
	mux.HandleFunc("/api/open", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeHTTP(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
			return
		}
		var request struct {
			Workspace string `json:"workspace"`
		}
		if err := decodeHTTP(r, &request); err != nil {
			writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		status, err := Inspect(request.Workspace)
		if err != nil {
			writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		m.setRoot(request.Workspace)
		writeHTTP(w, http.StatusOK, status)
	})
	mux.HandleFunc("/api/workspaces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeHTTP(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "GET required"})
			return
		}
		workspaces, err := m.listStagingWorkspaces()
		if err != nil {
			writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		writeHTTP(w, http.StatusOK, map[string]any{"status": "ok", "staging_root": m.stagingRoot, "workspaces": workspaces})
	})
	mux.HandleFunc("/api/witness/open", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeHTTP(w, http.StatusMethodNotAllowed, map[string]any{"status": "error", "error": "POST required"})
			return
		}
		var request struct {
			Workspace string `json:"workspace"`
		}
		if err := decodeHTTP(r, &request); err != nil {
			writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		root := strings.TrimSpace(request.Workspace)
		if root == "" {
			root = m.Root()
		}
		if err := m.requireStagingWorkspace(root); err != nil {
			writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		status, err := Inspect(root)
		if err != nil {
			writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		identity := status.Manifest.PluginIdentity
		if !strings.EqualFold(strings.TrimSpace(identity.Format), "VST3") || strings.TrimSpace(identity.InstallPath) == "" {
			writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": "selected staging workspace has no observed VST3 install path"})
			return
		}
		pid, err := m.startWitness(identity.InstallPath)
		if err != nil {
			writeHTTP(w, http.StatusBadRequest, map[string]any{"status": "error", "error": err.Error()})
			return
		}
		m.setRoot(root)
		writeHTTP(w, http.StatusOK, map[string]any{
			"status": "started", "workspace": root, "process_id": pid,
			"plugin_identity": identity,
			"notice":          "The GUI is an isolated witness instance. Closing it discards its state and does not issue a Credential or modify a Library, Catalog, or SPAL route.",
		})
	})
	mux.HandleFunc("/v1/workspace", func(w http.ResponseWriter, r *http.Request) {
		root := m.Root()
		if root == "" {
			writeHTTP(w, http.StatusOK, map[string]any{"status": "no_workspace"})
			return
		}
		Handler(root).ServeHTTP(w, r)
	})
	mux.HandleFunc("/v1/surface", m.forward)
	mux.HandleFunc("/v1/fxm", m.forward)
	mux.HandleFunc("/v1/probe", m.forward)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeHTTP(w, http.StatusOK, map[string]any{"status": "ok", "service": "vpsforge-desktop", "workspace": m.Root(), "staging_root": m.stagingRoot, "witness_available": m.witnessAvailable()})
	})
	return mux
}

func (m *DesktopManager) forward(w http.ResponseWriter, r *http.Request) {
	root := m.Root()
	if root == "" {
		writeHTTP(w, http.StatusConflict, map[string]any{"status": "error", "error": "open or create a workspace first"})
		return
	}
	Handler(root).ServeHTTP(w, r)
}

func (m *DesktopManager) Root() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.root
}

func (m *DesktopManager) setRoot(root string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.root = strings.TrimSpace(root)
}

func (m *DesktopManager) witnessAvailable() bool {
	if m.launchWitness != nil {
		return true
	}
	if strings.TrimSpace(m.witnessExecutable) == "" {
		return false
	}
	_, err := os.Stat(m.witnessExecutable)
	return err == nil
}

func (m *DesktopManager) requireStagingWorkspace(root string) error {
	root = strings.TrimSpace(root)
	if root == "" {
		return fmt.Errorf("select a staging workspace first")
	}
	staging := strings.TrimSpace(m.stagingRoot)
	if staging == "" {
		return fmt.Errorf("this desktop session has no configured staging root")
	}
	stagingAbsolute, err := filepath.Abs(staging)
	if err != nil {
		return fmt.Errorf("resolve staging root: %w", err)
	}
	workspaceAbsolute, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve workspace: %w", err)
	}
	relative, err := filepath.Rel(stagingAbsolute, workspaceAbsolute)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("witness may open only an isolated workspace below %s", stagingAbsolute)
	}
	return nil
}

func (m *DesktopManager) listStagingWorkspaces() ([]desktopWorkspace, error) {
	staging := strings.TrimSpace(m.stagingRoot)
	if staging == "" {
		return nil, fmt.Errorf("this desktop session has no configured staging root")
	}
	entries, err := os.ReadDir(staging)
	if err != nil {
		return nil, fmt.Errorf("read staging workspaces: %w", err)
	}
	latestByIdentity := map[string]desktopWorkspace{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		root := filepath.Join(staging, entry.Name())
		status, err := Inspect(root)
		if err != nil || status.Manifest.Status != "preflight_complete" {
			continue
		}
		identity := status.Manifest.PluginIdentity
		createdAt := status.Manifest.CreatedAt
		if createdAt.IsZero() {
			if info, infoErr := entry.Info(); infoErr == nil {
				createdAt = info.ModTime().UTC()
			}
		}
		workspace := desktopWorkspace{
			Workspace:       root,
			Manufacturer:    identity.Manufacturer,
			PluginName:      identity.Name,
			Format:          identity.Format,
			Version:         identity.Version,
			CandidateBadges: append([]string(nil), status.Manifest.CapabilityIDs...),
			CreatedAt:       createdAt,
			Launchable:      strings.EqualFold(identity.Format, "VST3") && strings.TrimSpace(identity.InstallPath) != "" && m.witnessAvailable(),
		}
		identityKey := strings.ToLower(strings.Join([]string{identity.Manufacturer, identity.Name, identity.Format, identity.Version}, "\x00"))
		if existing, ok := latestByIdentity[identityKey]; !ok || existing.CreatedAt.Before(workspace.CreatedAt) || (existing.CreatedAt.Equal(workspace.CreatedAt) && existing.Workspace < workspace.Workspace) {
			latestByIdentity[identityKey] = workspace
		}
	}
	workspaces := make([]desktopWorkspace, 0, len(latestByIdentity))
	for _, workspace := range latestByIdentity {
		workspaces = append(workspaces, workspace)
	}
	sort.Slice(workspaces, func(i, j int) bool {
		if workspaces[i].CreatedAt.Equal(workspaces[j].CreatedAt) {
			return workspaces[i].Workspace < workspaces[j].Workspace
		}
		return workspaces[i].CreatedAt.Before(workspaces[j].CreatedAt)
	})
	return workspaces, nil
}

func (m *DesktopManager) startWitness(pluginPath string) (int, error) {
	if m.launchWitness != nil {
		return m.launchWitness(pluginPath)
	}
	if !m.witnessAvailable() {
		return 0, fmt.Errorf("the independent VST3 witness executable is not available")
	}
	command := exec.Command(m.witnessExecutable,
		"--plugin-path", pluginPath,
		"--sample-rate", "48000",
		"--block-size", "512")
	command.Dir = filepath.Dir(pluginPath)
	if err := command.Start(); err != nil {
		return 0, fmt.Errorf("start independent VST3 witness: %w", err)
	}
	return command.Process.Pid, nil
}

const legacyDesktopHTML = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Vit VPS Forge</title><style>
body{font-family:Segoe UI,system-ui,sans-serif;background:#11151b;color:#edf2f7;margin:0}main{max-width:980px;margin:40px auto;padding:0 24px}section{background:#1a2029;border:1px solid #303a47;border-radius:12px;padding:20px;margin:16px 0}h1{font-size:26px}label{display:block;margin:10px 0 4px;color:#aeb9c7}input,select,button{font:inherit;border-radius:7px;border:1px solid #435064;padding:9px;background:#10151c;color:#fff}input{width:100%;box-sizing:border-box}button{cursor:pointer;background:#2a67d8;margin:12px 8px 0 0}button.secondary{background:#2d3744}pre{white-space:pre-wrap;background:#0c1117;padding:14px;border-radius:8px;max-height:360px;overflow:auto}.grid{display:grid;grid-template-columns:1fr 1fr;gap:14px}.note{color:#9eb0c5}@media(max-width:700px){.grid{grid-template-columns:1fr}}</style></head>
<body><main><h1>Vit VPS Forge</h1><p class="note">人工引导、Agent 协作、证据驱动的 VPS 锻造台。不会安装 VPS、签发 Credential 或修改 Catalog。</p>
<section><h2>打开已有工作区</h2><label>Workspace path</label><input id="openPath" placeholder="D:\\VPS-Authoring\\FabFilter-Pro-Q"><button class="secondary" onclick="openWorkspace()">打开</button></section>
<section><h2>创建工作区</h2><label>Workspace path</label><input id="workspace"><div class="grid"><div><label>Manufacturer</label><input id="manufacturer"></div><div><label>Plugin name</label><input id="name"></div><div><label>Format</label><select id="format"><option>VST3</option><option>CLAP</option><option>AU</option><option>AAX</option></select></div><div><label>Version</label><input id="version" value="unknown"></div></div><label>Capability IDs（逗号分隔；未知工牌可保留 unknown）</label><input id="capabilities" value="unknown"><button onclick="initWorkspace()">创建</button></section>
<section><h2>状态</h2><button class="secondary" onclick="refreshStatus()">刷新</button><pre id="status">尚未打开工作区。</pre></section>
<script>
async function post(url,body){const r=await fetch(url,{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(body)});const j=await r.json();if(!r.ok)throw new Error(j.error||r.statusText);return j}
function show(v){document.getElementById('status').textContent=JSON.stringify(v,null,2)}
async function openWorkspace(){try{show(await post('/api/open',{workspace:openPath.value}))}catch(e){show({error:e.message})}}
async function initWorkspace(){try{show(await post('/api/init',{workspace:workspace.value,manufacturer:manufacturer.value,name:name.value,format:format.value,version:version.value,capabilities:capabilities.value.split(',').map(x=>x.trim()).filter(Boolean)}))}catch(e){show({error:e.message})}}
async function refreshStatus(){try{show(await (await fetch('/v1/workspace')).json())}catch(e){show({error:e.message})}}
</script></main></body></html>`
