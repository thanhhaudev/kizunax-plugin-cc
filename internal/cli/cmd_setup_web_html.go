package cli

// setupFormHTML is the configuration form rendered at GET /.
// Pre-filled from existing config. Re-rendered on POST validation failure.
const setupFormHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>KizunaX setup</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; max-width: 640px; margin: 2rem auto; padding: 0 1rem; color: #1f2328; }
    h1 { font-size: 1.4rem; margin-bottom: 0.25rem; }
    p.lede { color: #57606a; margin-top: 0; }
    fieldset { border: 1px solid #d0d7de; border-radius: 6px; padding: 1rem 1.25rem; margin: 1rem 0; }
    legend { padding: 0 0.5rem; font-weight: 600; }
    label { display: block; margin: 0.6rem 0 0.2rem; font-size: 0.9rem; }
    input[type="text"], textarea, select { width: 100%; padding: 0.45rem 0.6rem; border: 1px solid #d0d7de; border-radius: 6px; font-size: 0.95rem; box-sizing: border-box; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
    textarea { min-height: 100px; resize: vertical; }
    input[type="radio"] { margin-right: 0.4rem; }
    .hint { font-size: 0.8rem; color: #57606a; margin-top: 0.2rem; }
    .err-banner { background: #ffebe9; border: 1px solid #ffc1bc; color: #82071e; padding: 0.6rem 0.9rem; border-radius: 6px; margin: 0.5rem 0 1rem; }
    .model-note { font-size: 0.8rem; color: #57606a; margin-top: 0.4rem; min-height: 1.2em; }
    button { background: #1f883d; color: #fff; border: 0; padding: 0.55rem 1.2rem; border-radius: 6px; font-size: 0.95rem; cursor: pointer; }
    button:hover { background: #1a7f37; }
    button.secondary { background: #f6f8fa; color: #1f2328; border: 1px solid #d0d7de; }
    button.secondary:hover { background: #eaeef2; }
  </style>
</head>
<body>
  <h1>Configure KizunaX</h1>
  <p class="lede">Paste one or more API keys. The plugin rotates through them on each request so quota is spread across keys.</p>

  {{if .Error}}<div class="err-banner">{{.Error}}</div>{{end}}

  <form method="POST" action="/save?t={{.Token}}">
    <fieldset>
      <legend>API keys</legend>
      <label>One key per line</label>
      <textarea name="api_keys" placeholder="kx_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"></textarea>
      <p class="hint">{{if gt .ExistingKeyCount 0}}{{.ExistingKeyCount}} key(s) currently saved. Leave empty to keep them.{{else}}At least one key is required.{{end}}</p>
    </fieldset>

    <fieldset>
      <legend>Models</legend>
      <p class="hint">Click <em>Load available models</em> to fetch the list using the first key above.</p>
      <button type="button" class="secondary" id="load-models">Load available models</button>
      <div class="model-note" id="model-note"></div>

      <label>OpenAI-compat model</label>
      <select name="openai_model" id="openai_model" disabled>
        <option value="{{.OpenAIModel}}" selected>{{.OpenAIModel}}</option>
      </select>

      <label>Anthropic-compat model</label>
      <select name="anthropic_model" id="anthropic_model" disabled>
        <option value="{{.AnthropicModel}}" selected>{{.AnthropicModel}}</option>
      </select>
    </fieldset>

    <fieldset>
      <legend>Rotation</legend>
      <label><input type="radio" name="rotation" value="round-robin" {{if eq .Rotation "round-robin"}}checked{{end}}> Round-robin (cycle through keys in order)</label>
      <p class="hint">Random rotation will arrive in a later release.</p>
    </fieldset>

    <button type="submit">Save</button>
  </form>

  <script>
    (function() {
      var btn = document.getElementById('load-models');
      var note = document.getElementById('model-note');
      var keysArea = document.querySelector('textarea[name="api_keys"]');
      var openaiSelect = document.getElementById('openai_model');
      var anthropicSelect = document.getElementById('anthropic_model');
      var token = '{{.Token}}';

      function firstKey() {
        var raw = (keysArea.value || '').split('\n');
        for (var i = 0; i < raw.length; i++) {
          var k = raw[i].trim();
          if (k) return k;
        }
        return '';
      }

      function fillSelect(sel, models, prev) {
        sel.innerHTML = '';
        var saw = false;
        models.forEach(function(m) {
          var opt = document.createElement('option');
          opt.value = m;
          opt.textContent = m;
          if (m === prev) { opt.selected = true; saw = true; }
          sel.appendChild(opt);
        });
        if (!saw && prev) {
          var opt = document.createElement('option');
          opt.value = prev;
          opt.textContent = prev + ' (saved)';
          opt.selected = true;
          sel.insertBefore(opt, sel.firstChild);
        }
      }

      function fetchModels(provider) {
        var body = new URLSearchParams();
        body.set('key', firstKey());
        return fetch('/list-models?t=' + encodeURIComponent(token) + '&provider=' + provider, {
          method: 'POST',
          headers: {'Content-Type': 'application/x-www-form-urlencoded'},
          body: body.toString(),
        }).then(function(r) {
          return r.json().then(function(j) { return {ok: r.ok, body: j}; });
        });
      }

      btn.addEventListener('click', function() {
        var k = firstKey();
        if (!k) { note.textContent = 'Paste at least one API key first.'; return; }
        note.textContent = 'Loading...';
        var prevOpenai = openaiSelect.value;
        var prevAnthropic = anthropicSelect.value;
        Promise.all([fetchModels('openai'), fetchModels('anthropic')]).then(function(results) {
          var r1 = results[0], r2 = results[1];
          var notes = [];
          if (r1.ok) {
            fillSelect(openaiSelect, r1.body.models || r1.body.fallback || [prevOpenai], prevOpenai);
            openaiSelect.disabled = false;
            if (r1.body.note) notes.push('openai: ' + r1.body.note);
          } else {
            notes.push('openai: ' + (r1.body && r1.body.error ? r1.body.error : 'failed'));
          }
          if (r2.ok) {
            fillSelect(anthropicSelect, r2.body.models || r2.body.fallback || [prevAnthropic], prevAnthropic);
            anthropicSelect.disabled = false;
            if (r2.body.note) notes.push('anthropic: ' + r2.body.note);
          } else {
            notes.push('anthropic: ' + (r2.body && r2.body.error ? r2.body.error : 'failed'));
          }
          note.textContent = notes.length ? notes.join('   ') : 'Models updated.';
        }).catch(function(e) {
          note.textContent = 'Failed: ' + e.message;
        });
      });
    })();
  </script>
</body>
</html>`

// setupSuccessHTML is rendered after a successful save. The binary exits ~2s after this is served.
const setupSuccessHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <title>KizunaX — saved</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; max-width: 640px; margin: 4rem auto; padding: 0 1rem; color: #1f2328; text-align: center; }
    h1 { color: #1f883d; }
    p { color: #57606a; }
  </style>
</head>
<body>
  <h1>Configuration saved</h1>
  <p>You can close this tab. The KizunaX CLI is exiting now.</p>
</body>
</html>`
