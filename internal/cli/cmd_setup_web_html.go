package cli

// setupFormHTML is the configuration form rendered at GET /.
// Pre-filled from existing config. Re-rendered on POST validation failure.
const setupFormHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Kizunax setup</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; max-width: 640px; margin: 2rem auto; padding: 0 1rem; color: #1f2328; }
    h1 { font-size: 1.4rem; margin-bottom: 0.25rem; }
    p.lede { color: #57606a; margin-top: 0; }
    fieldset { border: 1px solid #d0d7de; border-radius: 6px; padding: 1rem 1.25rem; margin: 1rem 0; }
    legend { padding: 0 0.5rem; font-weight: 600; }
    label { display: block; margin: 0.6rem 0 0.2rem; font-size: 0.9rem; }
    input[type="text"], input[type="url"], input[type="password"] { width: 100%; padding: 0.45rem 0.6rem; border: 1px solid #d0d7de; border-radius: 6px; font-size: 0.95rem; box-sizing: border-box; }
    input[type="checkbox"], input[type="radio"] { margin-right: 0.4rem; }
    .row-check { font-weight: 600; }
    .hint { font-size: 0.8rem; color: #57606a; margin-top: 0.2rem; }
    .err-banner { background: #ffebe9; border: 1px solid #ffc1bc; color: #82071e; padding: 0.6rem 0.9rem; border-radius: 6px; margin: 0.5rem 0 1rem; }
    button { background: #1f883d; color: #fff; border: 0; padding: 0.55rem 1.2rem; border-radius: 6px; font-size: 0.95rem; cursor: pointer; }
    button:hover { background: #1a7f37; }
    .row-default { margin-top: 0.6rem; }
    .row-default label { display: inline; margin-right: 1rem; font-weight: normal; }
  </style>
</head>
<body>
  <h1>Configure Kizunax</h1>
  <p class="lede">Fill the form below and click Save. The CLI exits once your config is written.</p>

  {{if .Error}}<div class="err-banner">{{.Error}}</div>{{end}}

  <form method="POST" action="/save?t={{.Token}}">
    <fieldset>
      <legend>OpenAI-compatible</legend>
      <label class="row-check"><input type="checkbox" name="openai_enabled" value="1" {{if .OpenAI.Enabled}}checked{{end}}> Configure this provider</label>
      <label>Base URL</label>
      <input type="url" name="openai_base_url" value="{{.OpenAI.BaseURL}}" placeholder="https://kizunax.io/api/coding/v1">
      <label>Model</label>
      <input type="text" name="openai_model" value="{{.OpenAI.Model}}" placeholder="coding/MiniMax-M2.7">
      <label>API key</label>
      <input type="password" name="openai_api_key" autocomplete="off" placeholder="{{if .OpenAI.HasKey}}(existing key — leave empty to keep){{else}}kx_...{{end}}">
    </fieldset>

    <fieldset>
      <legend>Anthropic-compatible</legend>
      <label class="row-check"><input type="checkbox" name="anthropic_enabled" value="1" {{if .Anthropic.Enabled}}checked{{end}}> Configure this provider</label>
      <label>Base URL</label>
      <input type="url" name="anthropic_base_url" value="{{.Anthropic.BaseURL}}" placeholder="https://kizunax.io/api/coding/anthropic/v1">
      <label>Model</label>
      <input type="text" name="anthropic_model" value="{{.Anthropic.Model}}" placeholder="MiniMax-M2.7-highspeed">
      <label>API key</label>
      <input type="password" name="anthropic_api_key" autocomplete="off" placeholder="{{if .Anthropic.HasKey}}(existing key — leave empty to keep){{else}}kx_...{{end}}">
      <label style="font-weight: normal; margin-top: 0.5rem;"><input type="checkbox" id="same_key" name="same_key" value="1" {{if .SameKey}}checked{{end}}> Use the same API key as OpenAI-compatible</label>
    </fieldset>

    <fieldset>
      <legend>Default provider</legend>
      <div class="row-default">
        <label><input type="radio" name="default_provider" value="openai" {{if eq .DefaultProvider "openai"}}checked{{end}}> openai</label>
        <label><input type="radio" name="default_provider" value="anthropic" {{if eq .DefaultProvider "anthropic"}}checked{{end}}> anthropic</label>
      </div>
      <p class="hint">Used when a command does not pass --provider.</p>
    </fieldset>

    <button type="submit">Save</button>
  </form>

  <script>
    (function() {
      var same = document.getElementById('same_key');
      if (!same) return;
      var openai = document.querySelector('input[name="openai_api_key"]');
      var anthro = document.querySelector('input[name="anthropic_api_key"]');
      function sync() {
        if (same.checked) {
          anthro.value = openai.value;
          anthro.setAttribute('readonly', 'readonly');
        } else {
          anthro.removeAttribute('readonly');
        }
      }
      same.addEventListener('change', sync);
      openai.addEventListener('input', function() { if (same.checked) anthro.value = openai.value; });
      sync();
    })();
  </script>
</body>
</html>`

// setupSuccessHTML is rendered after a successful save. The binary exits ~2s after this is served.
const setupSuccessHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <title>Kizunax — saved</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; max-width: 640px; margin: 4rem auto; padding: 0 1rem; color: #1f2328; text-align: center; }
    h1 { color: #1f883d; }
    p { color: #57606a; }
  </style>
</head>
<body>
  <h1>Configuration saved</h1>
  <p>You can close this tab. The Kizunax CLI is exiting now.</p>
</body>
</html>`
