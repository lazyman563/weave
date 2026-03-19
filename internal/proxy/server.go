package proxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type ProxyServer struct {
	Target   string
	Port     int
	LocalIP  string
}

func NewProxyServer(target string, port int) *ProxyServer {
	return &ProxyServer{
		Target:  target,
		Port:    port,
		LocalIP: getLocalIP(),
	}
}

func (p *ProxyServer) Address() string {
	return fmt.Sprintf("http://%s:%d", p.LocalIP, p.Port)
}

func (p *ProxyServer) WeaveAddress() string {
	return fmt.Sprintf("http://%s.weave.localhost:%d", p.LocalIP, p.Port)
}

func (p *ProxyServer) Start() error {
	targetURL, err := url.Parse(p.Target)
	if err != nil {
		return fmt.Errorf("URL alvo inválida: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		req.Host = targetURL.Host
		req.Header.Set("User-Agent", "github.com/lazyman563/weave/1.0")
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			return nil
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		resp.Body.Close()

		injected := injectOverlay(string(body), p.Target)
		resp.Body = io.NopCloser(strings.NewReader(injected))
		resp.ContentLength = int64(len(injected))
		resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(injected)))
		resp.Header.Del("Content-Security-Policy")
		resp.Header.Del("X-Frame-Options")
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/__weave__/fix", handleFix)
	mux.HandleFunc("/__weave__/elements", handleElements)
	mux.Handle("/", proxy)

	addr := fmt.Sprintf("0.0.0.0:%d", p.Port)
	log.Printf("weave serve escutando em %s", addr)
	return http.ListenAndServe(addr, mux)
}

func handleFix(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"fix endpoint ready"}`))
}

func handleElements(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"elements endpoint ready"}`))
}

func injectOverlay(html string, target string) string {
	overlay := fmt.Sprintf(`
<style>
  #weave-overlay {
    position: fixed; top: 0; left: 0; right: 0;
    background: #0a0a0a; color: #00FFB3;
    font-family: 'JetBrains Mono', monospace;
    font-size: 12px; padding: 6px 14px;
    z-index: 999999; display: flex; align-items: center;
    gap: 12px; border-bottom: 1px solid #00FFB330;
  }
  #weave-overlay span { color: #555; }
  #weave-overlay strong { color: #FFD700; }
  #weave-overlay button {
    background: #00FFB315; border: 1px solid #00FFB350;
    color: #00FFB3; padding: 2px 10px; border-radius: 4px;
    cursor: pointer; font-size: 11px;
  }
  #weave-overlay button:hover { background: #00FFB330; }
  .weave-highlight { outline: 2px solid #00FFB3 !important; }
  #weave-panel {
    position: fixed; bottom: 0; left: 0; right: 0;
    background: #0a0a0a; border-top: 1px solid #00FFB330;
    color: #ccc; font-family: monospace; font-size: 12px;
    padding: 10px 14px; z-index: 999998; display: none;
  }
  #weave-panel.open { display: block; }
  #weave-selector { color: #00FFB3; font-weight: bold; }
  #weave-instr {
    background: #111; border: 1px solid #333; color: #eee;
    width: 60%%; padding: 4px 8px; font-family: monospace;
    font-size: 12px; border-radius: 4px; margin: 0 8px;
  }
  #weave-send {
    background: #00FFB315; border: 1px solid #00FFB3;
    color: #00FFB3; padding: 4px 14px; cursor: pointer;
    border-radius: 4px;
  }
  #weave-result { margin-top: 8px; color: #FFD700; }
</style>
<div id="weave-overlay">
  ⚡ <strong>WEAVE</strong>
  <span>proxy →</span> %s
  <button onclick="toggleInspect()">🔍 Inspecionar</button>
  <button onclick="togglePanel()">🔧 Fix Panel</button>
  <span id="weave-mode" style="color:#FF4444">● inspect OFF</span>
</div>
<div id="weave-panel">
  <div>Elemento: <span id="weave-selector">nenhum selecionado</span></div>
  <div style="margin-top:6px">
    Instrução: <input id="weave-instr" placeholder="ex: esse botão deve chamar /auth/v2" />
    <button id="weave-send" onclick="sendFix()">Enviar para IA ⚡</button>
  </div>
  <div id="weave-result"></div>
</div>
<script>
  let inspecting = false;
  let selectedEl = null;

  function toggleInspect() {
    inspecting = !inspecting;
    const mode = document.getElementById('weave-mode');
    mode.textContent = inspecting ? '● inspect ON' : '● inspect OFF';
    mode.style.color = inspecting ? '#00FFB3' : '#FF4444';
    document.body.style.cursor = inspecting ? 'crosshair' : '';
  }

  function togglePanel() {
    document.getElementById('weave-panel').classList.toggle('open');
  }

  document.addEventListener('click', function(e) {
    if (!inspecting) return;
    if (e.target.closest('#weave-overlay') || e.target.closest('#weave-panel')) return;
    e.preventDefault(); e.stopPropagation();
    if (selectedEl) selectedEl.classList.remove('weave-highlight');
    selectedEl = e.target;
    selectedEl.classList.add('weave-highlight');
    const sel = getSelector(e.target);
    document.getElementById('weave-selector').textContent = sel;
    document.getElementById('weave-panel').classList.add('open');
  }, true);

  function getSelector(el) {
    if (el.id) return '#' + el.id;
    if (el.className) return el.tagName.toLowerCase() + '.' + [...el.classList].join('.');
    return el.tagName.toLowerCase();
  }

  function sendFix() {
    const sel = document.getElementById('weave-selector').textContent;
    const instr = document.getElementById('weave-instr').value;
    const result = document.getElementById('weave-result');
    if (!instr) { result.textContent = 'Digite uma instrução primeiro.'; return; }
    result.textContent = '⚡ Enviando para IA...';
    fetch('/__weave__/fix', {
      method: 'POST',
      headers: {'Content-Type':'application/json'},
      body: JSON.stringify({selector: sel, instruction: instr, html: selectedEl ? selectedEl.outerHTML : ''})
    })
    .then(r => r.json())
    .then(d => {
      result.textContent = '✅ ' + (d.result || 'Correção aplicada!');
      if (d.fixed_html && selectedEl) {
        const tmp = document.createElement('div');
        tmp.innerHTML = d.fixed_html;
        if (tmp.firstChild) selectedEl.replaceWith(tmp.firstChild);
      }
    })
    .catch(() => result.textContent = '❌ Erro ao conectar com weave fix');
  }
</script>
`, target)

	// Inject before </body>
	if idx := strings.LastIndex(html, "</body>"); idx != -1 {
		return html[:idx] + overlay + html[idx:]
	}
	return html + overlay
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip4 := ipnet.IP.To4(); ip4 != nil {
				return ip4.String()
			}
		}
	}
	return "127.0.0.1"
}
